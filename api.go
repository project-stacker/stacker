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
	DockerType = "docker"
	TarType    = "tar"
	OCIType    = "oci"
	BuiltType  = "built"
)

type ImageSource struct {
	Type string `yaml:"type"`
	Url  string `yaml:"url"`
	Tag  string `yaml:"tag"`
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
	Import      []string          `yaml:"import"`
	Run         interface{}       `yaml:"run"`
	Entrypoint  string            `yaml:"entrypoint"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []string          `yaml:"volumes"`
}

func (l *Layer) ParseEntrypoint() ([]string, error) {
	return shlex.Split(l.Entrypoint, true)
}

func (l *Layer) getRun() ([]string, error) {
	// This is how the json decoder decodes it if it's:
	// run:
	//     - foo
	//     - bar
	ifs, ok := l.Run.([]interface{})
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
	line, ok := l.Run.(string)
	if ok {
		return []string{line}, nil
	}

	// This is how it is after we do our find replace and re-set it; as a
	// convenience (so we don't have to re-wrap it in interface{}), let's
	// handle []string
	strs, ok := l.Run.([]string)
	if ok {
		return strs, nil
	}

	return nil, fmt.Errorf("unknown run directive type: %T", l.Run)
}

func NewStackerfile(stackerfile string) (Stackerfile, error) {
	sf := Stackerfile{}

	raw, err := ioutil.ReadFile(stackerfile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(raw, &sf); err != nil {
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

func (s *Stackerfile) VariableSub(from, to string) error {
	from = fmt.Sprintf("$%s", from)
	for _, layer := range *s {
		for i, imp := range layer.Import {
			layer.Import[i] = strings.Replace(imp, from, to, -1)
		}

		runs := []string{}
		old, err := layer.getRun()
		if err != nil {
			return err
		}

		for _, r := range old {
			runs = append(runs, strings.Replace(r, from, to, -1))
		}
		layer.Run = runs
	}

	return nil
}
