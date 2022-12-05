package stacker

import (
	"fmt"
	"os"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

type Stackerfile map[string]*types.Layer

type ConvertArgs struct {
	Config         types.StackerConfig
	Progress       bool
	InputFile      string
	OutputFile     string
	SubstituteFile string
}

// Converter is responsible for converting a Dockerfile into stackerfile
type Converter struct {
	opts      *ConvertArgs // Convert options
	output    Stackerfile
	currLayer string
	subs      map[string]string
	args      []string
}

// NewConverter initializes a new Converter struct
func NewConverter(opts *ConvertArgs) *Converter {
	return &Converter{
		opts:   opts,
		output: Stackerfile{},
		subs:   make(map[string]string),
		args:   []string{},
	}
}

func (c *Converter) Convert() error {
	if err := c.parseFile(); err != nil {
		log.Errorf("convert failed with err:%e", err)
		return err
	}

	return nil
}

type Command struct {
	Cmd       string   // lowercased command name (ex: `from`)
	SubCmd    string   // for ONBUILD only this holds the sub-command
	Json      bool     // whether the value is written in json form
	Original  string   // The original source line
	StartLine int      // The original source line number which starts this command
	EndLine   int      // The original source line number which ends this command
	Flags     []string // Any flags such as `--from=...` for `COPY`.
	Value     []string // The contents of the command (ex: `ubuntu:xenial`)
}

func (c *Converter) convertCommand(cmd *Command) error {
	var layer *types.Layer
	if c.currLayer != "" {
		layer = c.output[c.currLayer]
	}

	log.Debugf("cmd: %v", cmd)
	switch strings.ToLower(cmd.Cmd) {
	case "from":
		if len(cmd.Value) == 1 {
			c.currLayer = "${{IMAGE}}"
			c.subs["IMAGE"] = "app"
		} else if len(cmd.Value) == 3 && strings.EqualFold(cmd.Value[1], "as") {
			c.currLayer = cmd.Value[2]
		} else {
			return errors.Errorf("unsupported FROM directive")
		}
		layer := types.Layer{}
		if !strings.EqualFold(cmd.Value[0], "scratch") {
			layer.From.Type = "docker"
			layer.From.Url = fmt.Sprintf("docker://%s", cmd.Value[0])
		} else {
			layer.From.Type = cmd.Value[0]
		}
		c.output[c.currLayer] = &layer
	case "run":
		// replace any ARGs first
		for i, val := range cmd.Value {
			for _, arg := range c.args {
				repl := strings.ReplaceAll(val, fmt.Sprintf("$%s", arg), fmt.Sprintf("${{%s}}", arg))
				if repl != val {
					cmd.Value[i] = repl
					break
				}

				repl = strings.ReplaceAll(val, fmt.Sprintf("${%s}", arg), fmt.Sprintf("${{%s}}", arg))
				if repl != val {
					cmd.Value[i] = repl
					break
				}
			}
		}
		layer.Run = append(layer.Run, cmd.Value...)
	case "cmd":
		layer.Cmd = cmd.Value
	case "label":
		for _, label := range cmd.Value {
			parts := strings.Split(label, "=")
			layer.Labels[parts[0]] = parts[1]
		}
	case "maintainer": // ignored, deprecated
		if layer.Annotations == nil {
			layer.Annotations = map[string]string{}
		}

		key := "org.opencontainers.image.authors"
		if _, ok := layer.Annotations[key]; !ok {
			layer.Annotations[key] = cmd.Value[0]
		} else {
			layer.Annotations[key] += "," + cmd.Value[0]
		}
	case "expose": // ignored, runtime config
		return nil
	case "env":
		if layer.Environment == nil {
			layer.Environment = map[string]string{}
		}

		if len(cmd.Value) == 2 {
			val := cmd.Value[1]
			for _, arg := range c.args {
				repl := strings.ReplaceAll(val, fmt.Sprintf("$%s", arg), fmt.Sprintf("${{%s}}", arg))
				if repl != val {
					val = repl
					break
				}

				repl = strings.ReplaceAll(val, fmt.Sprintf("${%s}", arg), fmt.Sprintf("${{%s}}", arg))
				if repl != val {
					val = repl
					break
				}
			}

			layer.Environment[cmd.Value[0]] = val
		}
	case "workdir":
		layer.Run = append(layer.Run, "cd", cmd.Value[0])
	case "arg":
		if len(cmd.Value) != 1 {
			return errors.Errorf("invalid arg - %v", cmd.Value)
		}
		parts := strings.Split(cmd.Value[0], "=")
		if len(parts) == 1 {
			// no default value
			c.args = append(c.args, parts[0])
		} else if len(parts) == 2 {
			c.args = append(c.args, parts[0])
			c.subs[parts[0]] = parts[1]
		} else {
			return errors.Errorf("invalid arg - %v", cmd.Value)
		}
	case "copy":
		imp := types.Import{Path: cmd.Value[0], Dest: cmd.Value[1]}
		layer.Imports = append(layer.Imports, imp)
	case "volume":
		bind := types.Bind{Source: cmd.Value[0], Dest: cmd.Value[0]}
		layer.Binds = append(layer.Binds, bind)
		log.Infof("Bind-mounted volume %q found - make sure volume is present on host", cmd.Value[0])
	case "entrypoint":
		layer.Entrypoint = cmd.Value
	default:
		return errors.Errorf("unknown Dockerfile cmd: %s", cmd.Cmd)
	}

	return nil
}

func (c *Converter) parseFile() error {
	file, err := os.Open(c.opts.InputFile)
	if err != nil {
		log.Errorf("unable to open file %s", c.opts.InputFile)
		return err
	}
	defer file.Close()

	res, err := parser.Parse(file)
	if err != nil {
		log.Errorf("unable to parse file %s", c.opts.InputFile)
		return err
	}

	log.Infof("res: %v", res)

	for _, child := range res.AST.Children {
		cmd := Command{
			Cmd:       child.Value,
			Original:  child.Original,
			StartLine: child.StartLine,
			EndLine:   child.EndLine,
			Flags:     child.Flags,
		}

		// Only happens for ONBUILD
		if child.Next != nil && len(child.Next.Children) > 0 {
			cmd.SubCmd = child.Next.Children[0].Value
			child = child.Next.Children[0]
		}

		cmd.Json = child.Attributes["json"]
		for n := child.Next; n != nil; n = n.Next {
			cmd.Value = append(cmd.Value, n.Value)
		}

		if err := c.convertCommand(&cmd); err != nil {
			return err
		}
	}

	out, err := yaml.Marshal(c.output)
	if err != nil {
		return err
	}

	if err := os.WriteFile(c.opts.OutputFile, out, 0644); err != nil {
		return nil
	}

	// we have substitutions, so write that out also
	if len(c.subs) > 0 {
		out, err = yaml.Marshal(c.subs)
		if err != nil {
			return err
		}

		if err := os.WriteFile(c.opts.SubstituteFile, out, 0644); err != nil {
			return err
		}
	}

	return nil
}
