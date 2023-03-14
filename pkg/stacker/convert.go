package stacker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-shquot/shquot"
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
	vols      int
	args      []string
	env       map[string]string
	// per-layer state
	currDir string
	currUid string
	currGid string
}

// NewConverter initializes a new Converter struct
func NewConverter(opts *ConvertArgs) *Converter {
	return &Converter{
		opts:   opts,
		output: Stackerfile{},
		subs:   map[string]string{},
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

	log.Debugf("cmd: %+v", cmd)
	switch strings.ToLower(cmd.Cmd) {
	case "from":
		layer := types.Layer{BuildEnv: map[string]string{"arch": "x86_64"}}
		c.currDir = ""
		c.currUid = ""
		c.currGid = ""
		if len(cmd.Value) == 1 {
			c.currLayer = "${{IMAGE}}"
			c.subs["IMAGE"] = "app"
		} else if len(cmd.Value) == 3 && strings.EqualFold(cmd.Value[1], "as") {
			c.currLayer = cmd.Value[2]
			// layer.BuildOnly = true	// FIXME: should be enabled
		} else {
			return errors.Errorf("unsupported FROM directive")
		}
		if !strings.EqualFold(cmd.Value[0], "scratch") {
			layer.From.Type = "docker"
			layer.From.Url = fmt.Sprintf("docker://%s", cmd.Value[0])
		} else {
			layer.From.Type = cmd.Value[0]
		}
		c.output[c.currLayer] = &layer
	case "run":
		// setup the environment
		for k, v := range c.env {
			layer.Run = append(layer.Run, fmt.Sprintf("export %s=%s", k, v))
		}

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

				repl = strings.ReplaceAll(val, fmt.Sprintf("$(%s)", arg), fmt.Sprintf("${{%s}}", arg))
				if repl != val {
					cmd.Value[i] = repl
					break
				}
			}
		}

		for _, line := range cmd.Value {
			// patch some cmds
			re := regexp.MustCompile(`\bmkdir\b`)
			line = re.ReplaceAllString(line, "mkdir -p")

			if c.currUid == "" {
				// picking 'bash' here
				layer.Run = append(layer.Run, fmt.Sprintf("sh -e -c %s", shquot.POSIXShell([]string{line})))
			} else {
				layer.Run = append(layer.Run, fmt.Sprintf("su -p %s -c %s", c.currUid, shquot.POSIXShell([]string{line})))
			}
		}
	case "cmd":
		layer.Cmd = cmd.Value
	case "label":
		if layer.Labels == nil {
			layer.Labels = map[string]string{}
		}

		for i := 0; i < len(cmd.Value); i += 2 {
			layer.Labels[cmd.Value[i]] = cmd.Value[i+1]
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
		log.Infof("EXPOSE directive found - ports:%v", cmd.Value)
		return nil
	case "env":
		if len(cmd.Value) != 2 {
			log.Errorf("unable to parse ENV directive - %v", cmd.Original)
			return errors.Errorf("invalid arg - %v", cmd.Value)
		}

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

			repl = strings.ReplaceAll(val, fmt.Sprintf("$(%s)", arg), fmt.Sprintf("${{%s}}", arg))
			if repl != val {
				val = repl
				break
			}
		}

		if c.env == nil {
			c.env = map[string]string{}
		}

		c.env[cmd.Value[0]] = val
	case "workdir":
		layer.Run = append(layer.Run, fmt.Sprintf("cd %s", cmd.Value[0]))
		c.currDir = cmd.Value[0]
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
		// if --from is specified, then import, else just "cp"
		imp := types.Import{Path: cmd.Value[0]}
		dest := ""
		if len(cmd.Value) == 2 {
			dest = cmd.Value[1]
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(c.currDir, dest)
			}
		}
		imp.Dest = dest

		if len(cmd.Flags) > 0 {
			for _, flag := range cmd.Flags {
				if strings.HasPrefix(flag, "--from=") {
					layer := strings.TrimPrefix(flag, "--from=")
					imp.Path = fmt.Sprintf("stacker://%s", filepath.Join(layer, cmd.Value[0]))
				} else if strings.HasPrefix(flag, "--chown=") {
					mode := strings.TrimPrefix(flag, "--chown=")
					parts := strings.Split(mode, ":")
					uid, err := strconv.ParseInt(parts[0], 0, 32)
					if err != nil {
						log.Errorf("unable to parse COPY directive: %s", cmd.Original)
						return err
					}

					imp.Uid = int(uid)
					if len(parts) == 2 {
						gid, err := strconv.ParseInt(parts[1], 0, 32)
						if err != nil {
							log.Errorf("unable to parse COPY directive: %s", cmd.Original)
							return err
						}
						imp.Gid = int(gid)
					}
				}
			}
		}

		layer.Imports = append(layer.Imports, imp)
	case "volume":
		c.vols++
		vol := fmt.Sprintf("STACKER_VOL%d", c.vols)
		c.subs[vol] = cmd.Value[0]
		bind := types.Bind{Source: fmt.Sprintf("${{%s}}", vol), Dest: cmd.Value[0]}
		layer.Binds = append(layer.Binds, bind)
		log.Infof("Bind-mounted volume %q found (substituted via %s) - make sure volume is present on host", cmd.Value[0], vol)
	case "entrypoint":
		layer.Entrypoint = cmd.Value
	case "user":
		// su uid:gid
		parts := strings.Split(cmd.Value[0], ":")
		c.currUid = parts[0]
		if len(parts) == 2 {
			c.currGid = parts[1]
		}
	default:
		log.Errorf("unknown Dockerfile cmd: %s", cmd.Cmd)
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
