package stacker

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strings"

	"github.com/anmitsu/go-shlex"
	"gopkg.in/yaml.v2"
)

const (
	MediaTypeImageBtrfsLayer = "application/vnd.cisco.image.layer.btrfs"
)

// StackerConfig is a struct that contains global (or widely used) stacker
// config options.
type StackerConfig struct {
	StackerDir string
	OCIDir     string
	RootFSDir  string
}

type Stackerfile map[string]*Layer

const (
	DockerType  = "docker"
	TarType     = "tar"
	OCIType     = "oci"
	BuiltType   = "built"
	ScratchType = "scratch"
)

type ImageSource struct {
	Type     string `yaml:"type"`
	Url      string `yaml:"url"`
	Tag      string `yaml:"tag"`
	Insecure bool   `yaml:"insecure"`
}

func (is *ImageSource) ParseTag() (string, error) {
	switch is.Type {
	case BuiltType:
		return is.Tag, nil
	case DockerType:
		url, err := url.Parse(is.Url)
		if err != nil {
			return "", err
		}

		if url.Path != "" {
			tag := path.Base(strings.Replace(url.Path, ":", "-", -1))
			return tag, nil
		}

		// skopeo allows docker://centos:latest or
		// docker://docker.io/centos:latest; if we don't have a
		// url path, let's use the host as the image tag
		return strings.Replace(url.Host, ":", "-", -1), nil

	default:
		return "", fmt.Errorf("unsupported type: %s", is.Type)
	}
}

type Layer struct {
	From        *ImageSource      `yaml:"from"`
	Import      interface{}       `yaml:"import"`
	Run         interface{}       `yaml:"run"`
	Cmd         interface{}       `yaml:"cmd"`
	Entrypoint  interface{}       `yaml:"entrypoint"`
	FullCommand interface{}       `yaml:"full_command"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []string          `yaml:"volumes"`
	Labels      map[string]string `yaml:"labels"`
	WorkingDir  string            `yaml:"working_dir"`
	BuildOnly   bool              `yaml:"build_only"`
	Binds       interface{}       `yaml:"binds"`
}

func (l *Layer) ParseCmd() ([]string, error) {
	return l.getStringOrStringSlice(l.Cmd, func(s string) ([]string, error) {
		return shlex.Split(s, true)
	})
}

func (l *Layer) ParseEntrypoint() ([]string, error) {
	return l.getStringOrStringSlice(l.Entrypoint, func(s string) ([]string, error) {
		return shlex.Split(s, true)
	})
}

func (l *Layer) ParseFullCommand() ([]string, error) {
	return l.getStringOrStringSlice(l.FullCommand, func(s string) ([]string, error) {
		return shlex.Split(s, true)
	})
}

func (l *Layer) ParseImport() ([]string, error) {
	return l.getStringOrStringSlice(l.Import, func(s string) ([]string, error) {
		return strings.Split(s, "\n"), nil
	})
}

func (l *Layer) ParseBinds() ([]string, error) {
	return l.getStringOrStringSlice(l.Import, func(s string) ([]string, error) {
		return []string{s}, nil
	})
}

func (l *Layer) getRun() ([]string, error) {
	return l.getStringOrStringSlice(l.Run, func(s string) ([]string, error) {
		return []string{s}, nil
	})
}

func (l *Layer) getStringOrStringSlice(iface interface{}, xform func(string) ([]string, error)) ([]string, error) {
	// The user didn't supply run: at all, so let's not do anything.
	if iface == nil {
		return []string{}, nil
	}

	// This is how the json decoder decodes it if it's:
	// run:
	//     - foo
	//     - bar
	ifs, ok := iface.([]interface{})
	if ok {
		strs := []string{}
		for _, i := range ifs {
			s, ok := i.(string)
			if !ok {
				return nil, fmt.Errorf("unknown run array type: %T", i)
			}

			strs = append(strs, s)
		}
		return strs, nil
	}

	// This is how the json decoder decodes it if it's:
	// run: |
	//     echo hello world
	//     echo goodbye cruel world
	line, ok := iface.(string)
	if ok {
		return xform(line)
	}

	// This is how it is after we do our find replace and re-set it; as a
	// convenience (so we don't have to re-wrap it in interface{}), let's
	// handle []string
	strs, ok := iface.([]string)
	if ok {
		return strs, nil
	}

	return nil, fmt.Errorf("unknown directive type: %T", l.Run)
}

// NewStackerfile creates a new stackerfile from the given path. substitutions
// is a list of KEY=VALUE pairs of things to substitute. Note that this is
// explicitly not a map, because the substitutions are performed one at a time
// in the order that they are given.
func NewStackerfile(stackerfile string, substitutions []string) (Stackerfile, error) {
	sf := Stackerfile{}

	raw, err := ioutil.ReadFile(stackerfile)
	if err != nil {
		return nil, err
	}

	content := string(raw)

	for _, subst := range substitutions {
		membs := strings.SplitN(subst, "=", 2)
		if len(membs) != 2 {
			return nil, fmt.Errorf("invalid substition %s", subst)
		}

		from := fmt.Sprintf("$%s", membs[0])
		to := membs[1]

		fmt.Printf("substituting %s to %s\n", from, to)

		content = strings.Replace(content, from, to, -1)
	}

	if err := yaml.Unmarshal([]byte(content), &sf); err != nil {
		return nil, err
	}

	return sf, err
}

func (s *Stackerfile) DependencyOrder() ([]string, error) {
	ret := []string{}

	for i := 0; i < len(*s); i++ {
		for name, layer := range *s {
			have := false
			haveTag := false
			for _, l := range ret {
				if l == name {
					have = true
				}

				if l == layer.From.Tag {
					haveTag = true
				}
			}

			// do we have this layer yet?
			if !have {
				if layer.From == nil {
					return nil, fmt.Errorf("invalid layer: no base (from directive)")
				}
				// all imported layers have no deps
				if layer.From.Type != BuiltType {
					ret = append(ret, name)
				}

				// otherwise, we need to have the tag
				if haveTag {
					ret = append(ret, name)
				}
			}
		}
	}

	if len(ret) != len(*s) {
		return nil, fmt.Errorf("couldn't resolve some dependencies")
	}

	return ret, nil
}
