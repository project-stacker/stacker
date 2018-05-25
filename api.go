package stacker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"reflect"
	"regexp"
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

type Stackerfile struct {
	// internal is the actual representation of the stackerfile as a map.
	internal map[string]*Layer

	// fileOrder is the order of elements as they appear in the stackerfile.
	fileOrder []string
}

func (sf *Stackerfile) Get(name string) (*Layer, bool) {
	// This is dumb, but if we do a direct return here, golang doesn't
	// resolve the "ok", and compilation fails.
	layer, ok := sf.internal[name]
	return layer, ok
}

func (sf *Stackerfile) Len() int {
	return len(sf.internal)
}

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
			return path.Base(strings.Split(url.Path, ":")[0]), nil
		}

		// skopeo allows docker://centos:latest or
		// docker://docker.io/centos:latest; if we don't have a
		// url path, let's use the host as the image tag
		return strings.Split(url.Host, ":")[0], nil
	case OCIType:
		pieces := strings.Split(is.Url, ":")
		if len(pieces) != 2 {
			return "", fmt.Errorf("bad OCI tag: %s", is.Type)
		}

		return pieces[1], nil
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
	Apply       []string          `yaml:"apply"`
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
	return l.getStringOrStringSlice(l.Binds, func(s string) ([]string, error) {
		return []string{s}, nil
	})
}

func (l *Layer) ParseRun() ([]string, error) {
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

var (
	layerFields       []string
	imageSourceFields []string
)

func init() {
	layerFields = []string{}
	layerType := reflect.TypeOf(Layer{})
	for i := 0; i < layerType.NumField(); i++ {
		tag := layerType.Field(i).Tag.Get("yaml")
		layerFields = append(layerFields, tag)
	}

	imageSourceFields = []string{}
	imageSourceType := reflect.TypeOf(ImageSource{})
	for i := 0; i < imageSourceType.NumField(); i++ {
		tag := imageSourceType.Field(i).Tag.Get("yaml")
		imageSourceFields = append(imageSourceFields, tag)
	}
}

func substitute(content string, substitutions []string) (string, error) {
	for _, subst := range substitutions {
		membs := strings.SplitN(subst, "=", 2)
		if len(membs) != 2 {
			return "", fmt.Errorf("invalid substition %s", subst)
		}

		from := fmt.Sprintf("$%s", membs[0])
		to := membs[1]

		fmt.Printf("substituting %s to %s\n", from, to)

		content = strings.Replace(content, from, to, -1)

		re, err := regexp.Compile(fmt.Sprintf(`\$\{%s[^\}]*\}`, membs[0]))
		if err != nil {
			return "", err
		}

		content = re.ReplaceAllString(content, to)
		fmt.Println(content)
	}

	// now, anything that's left we can just use its value
	re, err := regexp.Compile("\\$\\{[^\\}]*\\}")
	indexes := re.FindAllStringIndex(content, -1)
	for _, idx := range indexes {
		// get content without ${}
		variable := content[idx[0]+2 : idx[1]-1]

		membs := strings.SplitN(variable, ":", 2)
		if len(membs) != 2 {
			return "", fmt.Errorf("no value for substitution %s", variable)
		}

		buf := bytes.NewBufferString(content[:idx[0]])
		_, err = buf.WriteString(membs[1])
		if err != nil {
			return "", err
		}
		_, err = buf.WriteString(content[idx[1]:])
		if err != nil {
			return "", err
		}

		content = buf.String()
	}

	return content, nil
}

// NewStackerfile creates a new stackerfile from the given path. substitutions
// is a list of KEY=VALUE pairs of things to substitute. Note that this is
// explicitly not a map, because the substitutions are performed one at a time
// in the order that they are given.
func NewStackerfile(stackerfile string, substitutions []string) (*Stackerfile, error) {
	sf := Stackerfile{}

	raw, err := ioutil.ReadFile(stackerfile)
	if err != nil {
		return nil, err
	}

	content, err := substitute(string(raw), substitutions)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(content), &sf.internal); err != nil {
		return nil, err
	}

	// Parse a second time so that we can remember the file order.
	ms := yaml.MapSlice{}
	if err := yaml.Unmarshal([]byte(content), &ms); err != nil {
		return nil, err
	}

	sf.fileOrder = []string{}
	for _, e := range ms {
		sf.fileOrder = append(sf.fileOrder, e.Key.(string))
	}

	// Now, let's make sure that all the things people supplied are
	// actually things this stacker understands.
	for _, e := range ms {
		for _, directive := range e.Value.(yaml.MapSlice) {
			found := false
			for _, field := range layerFields {
				if directive.Key.(string) == field {
					found = true
					break
				}
			}

			if !found {
				return nil, fmt.Errorf("unknown directive %s", directive.Key.(string))
			}

			if directive.Key.(string) == "from" {
				for _, sourceDirective := range directive.Value.(yaml.MapSlice) {
					found = false
					for _, field := range imageSourceFields {
						if sourceDirective.Key.(string) == field {
							found = true
							break
						}
					}

					if !found {
						return nil, fmt.Errorf("unknown image source directive %s", sourceDirective.Key.(string))
					}
				}
			}
		}
	}

	return &sf, err
}

func (s *Stackerfile) DependencyOrder() ([]string, error) {
	ret := []string{}
	processed := map[string]bool{}

	for i := 0; i < s.Len(); i++ {
		for _, name := range s.fileOrder {
			_, ok := processed[name]
			if ok {
				continue
			}

			layer := s.internal[name]

			if layer.From == nil {
				return nil, fmt.Errorf("invalid layer: no base (from directive)")
			}

			_, haveBaseTag := processed[layer.From.Tag]

			imports, err := layer.ParseImport()
			if err != nil {
				return nil, err
			}

			haveStackerImports := true
			for _, imp := range imports {
				url, err := url.Parse(imp)
				if err != nil {
					return nil, err
				}

				if url.Scheme != "stacker" {
					continue
				}

				_, ok := processed[url.Host]
				if !ok {
					haveStackerImports = false
					break
				}
			}

			// all imported layers have no deps, or if it's not
			// imported and we have the base tag, that's ok too.
			if haveStackerImports && (layer.From.Type != BuiltType || haveBaseTag) {
				ret = append(ret, name)
				processed[name] = true
			}
		}
	}

	if len(ret) != s.Len() {
		return nil, fmt.Errorf("couldn't resolve some dependencies")
	}

	return ret, nil
}
