package types

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	DockerLayer = "docker"
	TarLayer    = "tar"
	OCILayer    = "oci"
	BuiltLayer  = "built"
)

func IsContainersImageLayer(from string) bool {
	switch from {
	case DockerLayer:
		return true
	case OCILayer:
		return true
	}

	return false
}

type Import struct {
	Path string `yaml:"path"`
	Hash string `yaml:"hash"`
}

type OverlayDir struct {
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
}

type Imports []Import

func getStringOrStringSlice(iface interface{}, xform func(string) ([]string, error)) ([]string, error) {
	// The user didn't supply run: at all, so let's not do anything.
	if iface == nil {
		return []string{}, nil
	}

	// This is how the yaml decoder decodes it if it's:
	// run:
	//     - foo
	//     - bar
	ifs, ok := iface.([]interface{})
	if ok {
		strs := []string{}
		for _, i := range ifs {
			s, ok := i.(string)
			if !ok {
				return nil, errors.Errorf("unknown run array type: %T", i)
			}

			strs = append(strs, s)
		}
		return strs, nil
	}

	// This is how the yaml decoder decodes it if it's:
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

	return nil, errors.Errorf("unknown directive type: %T", iface)
}

// StringList allows this type to be parsed from the yaml parser as either a
// list of strings, or if the entry was just one string, it is a list of length
// one containing that string.
type StringList []string

func (sol *StringList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}
	err := unmarshal(&data)
	if err != nil {
		return errors.WithStack(err)
	}
	xform := func(s string) ([]string, error) {
		return []string{s}, nil
	}

	*sol, err = getStringOrStringSlice(data, xform)
	return err
}

// Command allows this type to be parsed from the yaml parser as either a list
// of strings or if a single string is specified, it is split with
// shlex.Split() into a list.
type Command []string

func (c *Command) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}
	err := unmarshal(&data)
	if err != nil {
		return errors.WithStack(err)
	}
	xform := func(s string) ([]string, error) {
		return shlex.Split(s, true)
	}

	result, err := getStringOrStringSlice(data, xform)
	if err != nil {
		return err
	}

	*c = Command(result)
	return nil
}

type Bind struct {
	Source string
	Dest   string
}

type Binds []Bind

func (bs *Binds) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}
	err := unmarshal(&data)
	if err != nil {
		return errors.WithStack(err)
	}

	xform := func(s string) ([]string, error) {
		return []string{s}, nil
	}

	rawBinds, err := getStringOrStringSlice(data, xform)
	if err != nil {
		return err
	}

	*bs = Binds{}
	for _, bind := range rawBinds {
		parts := strings.Split(bind, "->")
		if len(parts) != 1 && len(parts) != 2 {
			return errors.Errorf("invalid bind mount %s", bind)
		}

		source := strings.TrimSpace(parts[0])
		target := source

		if len(parts) == 2 {
			target = strings.TrimSpace(parts[1])
		}

		*bs = append(*bs, Bind{Source: source, Dest: target})
	}

	return nil
}

type Layer struct {
	From           ImageSource       `yaml:"from"`
	Imports        Imports           `yaml:"import"`
	OverlayDirs    []OverlayDir      `yaml:"overlay_dirs"`
	Run            StringList        `yaml:"run"`
	Cmd            Command           `yaml:"cmd"`
	Entrypoint     Command           `yaml:"entrypoint"`
	FullCommand    Command           `yaml:"full_command"`
	BuildEnvPt     []string          `yaml:"build_env_passthrough"`
	BuildEnv       map[string]string `yaml:"build_env"`
	Environment    map[string]string `yaml:"environment"`
	Volumes        []string          `yaml:"volumes"`
	Labels         map[string]string `yaml:"labels"`
	GenerateLabels StringList        `yaml:"generate_labels"`
	WorkingDir     string            `yaml:"working_dir"`
	BuildOnly      bool              `yaml:"build_only"`
	Binds          Binds             `yaml:"binds"`
	RuntimeUser    string            `yaml:"runtime_user"`
	Annotations    map[string]string `yaml:"annotations"`
	OS             *string           `yaml:"os"`
	Arch           *string           `yaml:"arch"`
}

func parseLayers(referenceDirectory string, lms yaml.MapSlice, requireHash bool) (map[string]Layer, error) {
	// Let's make sure that all the things people supplied in the layers are
	// actually things this stacker understands.
	for _, e := range lms {
		for _, directive := range e.Value.(yaml.MapSlice) {
			found := false
			for _, field := range layerFields {
				if directive.Key.(string) == field {
					found = true
					break
				}
			}

			if !found {
				return nil, errors.Errorf("stackerfile: unknown directive %s", directive.Key.(string))
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
						return nil, errors.Errorf("stackerfile: unknown image source directive %s",
							sourceDirective.Key.(string))
					}
				}
			}

			if directive.Key.(string) == "os" || directive.Key.(string) == "arch" {
				if directive.Value == nil {
					return nil, errors.Errorf("stackerfile: %q value cannot be empty", directive.Key.(string))
				}
			}
		}
	}

	// Marshall only the layers so we can unmarshal them in the right data structure later
	layersContent, err := yaml.Marshal(lms)
	if err != nil {
		return nil, err
	}

	ret := map[string]Layer{}
	// Unmarshal to save the data in the right structure to enable further processing
	if err := yaml.Unmarshal(layersContent, &ret); err != nil {
		return nil, err
	}

	for name, layer := range ret {
		if requireHash {
			err = requireImportHash(layer.Imports)
			if err != nil {
				return nil, err
			}
		}

		switch layer.From.Type {
		case BuiltLayer:
			if len(layer.From.Tag) == 0 {
				return nil, errors.Errorf("%s: from tag cannot be empty for image type 'built'", name)
			}
		}

		if layer.OS == nil {
			// if not specified, default to runtime
			os := runtime.GOOS
			layer.OS = &os
		}

		if layer.Arch == nil {
			// if not specified, default to runtime
			arch := runtime.GOARCH
			layer.Arch = &arch
		}

		ret[name], err = layer.absolutify(referenceDirectory)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (l Layer) absolutify(referenceDirectory string) (Layer, error) {
	getAbsPath := func(path string) (string, error) {
		parsedPath, err := NewDockerishUrl(path)
		if err != nil {
			return "", err
		}

		if parsedPath.Scheme != "" || filepath.IsAbs(path) {
			// Path is already absolute or is an URL, return it
			return path, nil
		} else {
			// If path is relative we need to add it to the directory where this layer is found
			abs, err := filepath.Abs(filepath.Join(referenceDirectory, path))
			return abs, errors.WithStack(err)
		}
	}

	ret := l

	ret.Imports = nil
	for _, rawImport := range l.Imports {
		absImportPath, err := getAbsPath(rawImport.Path)
		if err != nil {
			return ret, err
		}
		absImport := Import{Hash: rawImport.Hash, Path: absImportPath}
		ret.Imports = append(ret.Imports, absImport)
	}

	ret.OverlayDirs = nil
	for _, rawOverlayDir := range l.OverlayDirs {
		absSource, err := getAbsPath(rawOverlayDir.Source)
		if err != nil {
			return ret, err
		}

		od := OverlayDir{Source: absSource, Dest: rawOverlayDir.Dest}
		ret.OverlayDirs = append(ret.OverlayDirs, od)
	}

	ret.Binds = nil
	for _, rawBind := range l.Binds {
		absSource, err := getAbsPath(rawBind.Source)
		if err != nil {
			return ret, err
		}

		absDest, err := getAbsPath(rawBind.Dest)
		if err != nil {
			return ret, err
		}
		b := Bind{Source: absSource, Dest: absDest}
		ret.Binds = append(ret.Binds, b)
	}

	return ret, nil
}

func requireImportHash(imports Imports) error {
	for _, imp := range imports {
		url, err := NewDockerishUrl(imp.Path)
		if err != nil {
			return err
		}
		if (url.Scheme == "http" || url.Scheme == "https") && imp.Hash == "" {
			return errors.Errorf("Remote import needs a hash in yaml for path: %s", imp.Path)
		}
	}
	return nil
}

func getImportFromInterface(v interface{}) (Import, error) {
	m, ok := v.(map[interface{}]interface{})
	var hash string
	if ok {
		// check for nil hash so that we won't end up with "nil" string values
		if m["hash"] == nil {
			hash = ""
		} else {
			hash = fmt.Sprintf("%v", m["hash"])
		}
		return Import{Hash: hash, Path: fmt.Sprintf("%v", m["path"])}, nil
	}

	m2, ok := v.(map[string]interface{})
	if ok {
		// check for nil hash so that we won't end up with "nil" string values
		if m2["hash"] == nil {
			hash = ""
		} else {
			hash = fmt.Sprintf("%v", m2["hash"])
		}
		return Import{Hash: hash, Path: fmt.Sprintf("%v", m2["Path"])}, nil
	}

	// if it's not a map then it's a string
	s, ok := v.(string)
	if ok {
		return Import{Hash: "", Path: fmt.Sprintf("%v", s)}, nil
	}
	return Import{}, errors.Errorf("Didn't find a matching type for: %#v", v)
}

// Custom UnmarshalYAML from string/map/slice of strings/slice of maps into Imports
func (im *Imports) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}
	if err := unmarshal(&data); err != nil {
		return err
	}

	imports, ok := data.([]interface{})
	if ok {
		// imports are a list of either strings or maps
		for _, v := range imports {
			imp, err := getImportFromInterface(v)
			if err != nil {
				return err
			}
			*im = append(*im, imp)
		}
	} else {
		if data != nil {
			// import are either string or map
			imp, err := getImportFromInterface(data)
			if err != nil {
				return err
			}
			*im = append(*im, imp)
		}
	}

	return nil
}

func filterEnv(matchList []string, curEnv map[string]string) (map[string]string, error) {
	// matchList is a list of regular expressions.
	// curEnv is a map[string]string.
	// return is filtered set of curEnv that match an entry in matchList
	var err error
	var r *regexp.Regexp
	newEnv := map[string]string{}
	matches := []*regexp.Regexp{}
	for _, t := range matchList {
		r, err = regexp.Compile("^" + t + "$")
		if err != nil {
			return newEnv, err
		}
		matches = append(matches, r)
	}
	for key, val := range curEnv {
		for _, match := range matches {
			if match.Match([]byte(key)) {
				newEnv[key] = val
				break
			}
		}
	}
	return newEnv, err
}

func buildEnv(passThrough []string, newEnv map[string]string,
	getCurEnv func() []string) (map[string]string, error) {
	// get a map[string]string that should be used for the environment
	// of the container.
	curEnv := map[string]string{}
	for _, kv := range getCurEnv() {
		pair := strings.SplitN(kv, "=", 2)
		curEnv[pair[0]] = pair[1]
	}
	defList := []string{
		"ftp_proxy", "http_proxy", "https_proxy", "no_proxy",
		"FTP_PROXY", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "TERM"}
	matchList := defList
	if len(passThrough) != 0 {
		matchList = passThrough
	}
	ret, err := filterEnv(matchList, curEnv)
	if err != nil {
		return ret, err
	}
	for k, v := range newEnv {
		ret[k] = v
	}
	return ret, nil
}

func (l *Layer) BuildEnvironment(name string) (map[string]string, error) {
	env, err := buildEnv(l.BuildEnvPt, l.BuildEnv, os.Environ)
	env["STACKER_LAYER_NAME"] = name
	return env, err
}

var (
	layerFields []string
)

func init() {
	layerFields = []string{}
	layerType := reflect.TypeOf(Layer{})
	for i := 0; i < layerType.NumField(); i++ {
		tag := layerType.Field(i).Tag.Get("yaml")
		// some fields are ",omitempty"
		tag = strings.Split(tag, ",")[0]
		layerFields = append(layerFields, tag)
	}
}
