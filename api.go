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

		tag := path.Base(strings.Replace(url.Path, ":", "-", -1))
		if tag != "" {
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
	From       *ImageSource `yaml:"from"`
	Import     []string     `yaml:"import"`
	Run        []string     `yaml:"run"`
	Entrypoint string       `yaml:"entrypoint"`
}

func (l *Layer) ParseEntrypoint() ([]string, error) {
	return shlex.Split(l.Entrypoint, true)
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

func (s *Stackerfile) VariableSub(from, to string) {
	from = fmt.Sprintf("$%s", from)
	for _, layer := range *s {
		for i, imp := range layer.Import {
			layer.Import[i] = strings.Replace(imp, from, to, -1)
		}

		for i, r := range layer.Run {
			layer.Run[i] = strings.Replace(r, from, to, -1)
		}
	}
}
