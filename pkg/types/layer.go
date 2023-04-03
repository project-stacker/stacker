package types

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"stackerbuild.io/stacker/pkg/lib"
)

const (
	DockerLayer  = "docker"
	TarLayer     = "tar"
	OCILayer     = "oci"
	BuiltLayer   = "built"
	ScratchLayer = "scratch"
)

const (
	AuthorAnnotation  = "org.opencontainers.image.authors"
	OrgAnnotation     = "org.opencontainers.image.vendor"
	LicenseAnnotation = "org.opencontainers.image.licenses"
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
	Path string       `yaml:"path" json:"path"`
	Hash string       `yaml:"hash" json:"hash,omitempty"`
	Dest string       `yaml:"dest" json:"dest,omitempty"`
	Mode *fs.FileMode `yaml:"mode" json:"mode,omitempty"`
	Uid  int          `yaml:"uid" json:"uid,omitempty"`
	Gid  int          `yaml:"gid" json:"gid,omitempty"`
}

type Imports []Import

type OverlayDir struct {
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
}

type OverlayDirs []OverlayDir

type Package struct {
	Name    string
	Version string
	License string
	Paths   []string
}

type Bom struct {
	Generate bool      `yaml:"generate" json:"generate"`
	Packages []Package `yaml:"packages" json:"packages,omitempty"`
}

func getStringOrStringSlice(data interface{}, xform func(string) ([]string, error)) ([]string, error) {
	// The user didn't supply run: at all, so let's not do anything.
	if data == nil {
		return []string{}, nil
	}

	// This is how the yaml decoder decodes it if it's:
	// run:
	//     - foo
	//     - bar
	ifs, ok := data.([]interface{})
	if ok {
		strs := []string{}
		for _, i := range ifs {
			s := ""
			switch v := i.(type) {
			case string:
				s = v
			default:
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
	line, ok := data.(string)
	if ok {
		return xform(line)
	}

	// This is how it is after we do our find replace and re-set it; as a
	// convenience (so we don't have to re-wrap it in interface{}), let's
	// handle []string
	strs, ok := data.([]string)
	if ok {
		return strs, nil
	}

	return nil, errors.Errorf("unknown directive type: %T", data)
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

type bindType struct {
	Source string `yaml:"source" json:"source,omitempty"`
	Dest   string `yaml:"dest" json:"dest,omitempty"`
}

// toBind - copy to Bind type and check.
func (b *bindType) toBind(bs *Bind) error {
	if b.Source == "" {
		return errors.Errorf("unexpected 'bind': missing required field 'source': %#v", b)
	}
	bs.Source = b.Source
	bs.Dest = b.Dest
	if bs.Dest == "" {
		bs.Dest = bs.Source
	}
	return nil
}

func (b *bindType) toBindFromString(bind *Bind, asStr string) error {
	toks := strings.Fields(asStr)
	if len(toks) == 1 {
		bind.Source = toks[0]
		bind.Dest = toks[0]
		return nil
	} else if len(toks) == 3 && toks[1] == "->" {
		bind.Source = toks[0]
		bind.Dest = toks[2]
		return nil
	}
	return errors.Errorf("invalid Bind: %s", string(asStr))
}

type Bind struct {
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	Dest   string `yaml:"dest,omitempty" json:"dest,omitempty"`
}

type Binds []Bind

func (bs *Bind) UnmarshalJSON(data []byte) error {
	btype := bindType{}
	if err := json.Unmarshal(data, &btype); err == nil {
		return btype.toBind(bs)
	}

	asStr := ""
	err := json.Unmarshal(data, &asStr)
	if err != nil {
		return errors.Errorf("invalid Bind: %s", string(data))
	}

	return btype.toBindFromString(bs, asStr)
}

func (bs *Bind) UnmarshalYAML(unmarshal func(interface{}) error) error {
	btype := bindType{}
	if err := unmarshal(&btype); err == nil {
		return btype.toBind(bs)
	}

	asStr := ""
	if err := unmarshal(&asStr); err == nil {
		return btype.toBindFromString(bs, asStr)
	}

	var data interface{}
	if err := unmarshal(&data); err != nil {
		return errors.Errorf("unexpected error unmarshaling bind yaml: %v", err)
	}

	return errors.Errorf("unexpected 'bind' data of type: %s: %#v", reflect.TypeOf(data), data)
}

type Layer struct {
	From           ImageSource       `yaml:"from" json:"from"`
	Imports        Imports           `yaml:"import" json:"import,omitempty"`
	OverlayDirs    OverlayDirs       `yaml:"overlay_dirs" json:"overlay_dirs,omitempty"`
	Run            StringList        `yaml:"run" json:"run,omitempty"`
	Cmd            Command           `yaml:"cmd" json:"cmd,omitempty"`
	Entrypoint     Command           `yaml:"entrypoint" json:"entrypoint,omitempty"`
	FullCommand    Command           `yaml:"full_command" json:"full_command,omitempty"`
	BuildEnvPt     []string          `yaml:"build_env_passthrough" json:"build_env_passthrough,omitempty"`
	BuildEnv       map[string]string `yaml:"build_env" json:"build_env,omitempty"`
	Environment    map[string]string `yaml:"environment" json:"environment,omitempty"`
	Volumes        []string          `yaml:"volumes" json:"volumes,omitempty"`
	Labels         map[string]string `yaml:"labels" json:"labels,omitempty"`
	GenerateLabels StringList        `yaml:"generate_labels" json:"generate_labels,omitempty"`
	WorkingDir     string            `yaml:"working_dir" json:"working_dir,omitempty"`
	BuildOnly      bool              `yaml:"build_only" json:"build_only,omitempty"`
	Binds          Binds             `yaml:"binds" json:"binds,omitempty"`
	RuntimeUser    string            `yaml:"runtime_user" json:"runtime_user,omitempty"`
	Annotations    map[string]string `yaml:"annotations" json:"annotations,omitempty"`
	OS             *string           `yaml:"os" json:"os,omitempty"`
	Arch           *string           `yaml:"arch" json:"arch,omitempty"`
	Bom            *Bom              `yaml:"bom" json:"bom,omitempty"`
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

		if layer.Bom != nil && layer.Bom.Generate {
			if layer.Annotations == nil {
				return nil, errors.Errorf("for bom generation %s, %s and %s annotations must be set",
					AuthorAnnotation, OrgAnnotation, LicenseAnnotation)
			}

			_, aok := layer.Annotations[AuthorAnnotation]
			_, ook := layer.Annotations[OrgAnnotation]
			_, lok := layer.Annotations[LicenseAnnotation]
			if !aok || !ook || !lok {
				return nil, errors.Errorf("for bom generation %s, %s and %s annotations must be set",
					AuthorAnnotation, OrgAnnotation, LicenseAnnotation)
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
		if rawImport.Path[len(rawImport.Path)-1:] == "/" {
			absImportPath += "/"
		}
		absImport := Import{Hash: rawImport.Hash, Path: absImportPath, Dest: rawImport.Dest, Mode: rawImport.Mode, Uid: rawImport.Uid, Gid: rawImport.Gid}
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

// getImportFromInterface -
//
//	 an Import (an entry in 'imports'), can be written in yaml as either a string or a map[string]:
//	  imports:
//	   - /path/to-file
//	   - path: /path/f2
//	This function gets a single entry in that list and returns the Import.
func getImportFromInterface(v interface{}) (Import, error) {
	mode := -1
	ret := Import{Mode: nil, Uid: lib.UidEmpty, Gid: lib.GidEmpty}

	// if it is a simple string, that is the path
	s, ok := v.(string)
	if ok {
		ret.Path = s
		return ret, nil
	}

	m, ok := v.(map[interface{}]interface{})
	if !ok {
		return Import{}, errors.Errorf("could not read imports entry: %#v", v)
	}

	for k := range m {
		if _, ok := k.(string); !ok {
			return Import{}, errors.Errorf("key '%s' in import is not a string: %#v", k, v)
		}
	}

	// if present, these must have string values.
	for name, dest := range map[string]*string{"hash": &ret.Hash, "path": &ret.Path, "dest": &ret.Dest} {
		val, found := m[name]
		if !found {
			continue
		}
		s, ok := val.(string)
		if !ok {
			return Import{}, errors.Errorf("value for '%s' in import is not a string: %#v", name, v)
		}
		*dest = s
	}

	// if present, these must have int values
	for name, dest := range map[string]*int{"mode": &mode, "uid": &ret.Uid, "gid": &ret.Gid} {
		val, found := m[name]
		if !found {
			continue
		}
		i, ok := val.(int)
		if !ok {
			return Import{}, errors.Errorf("value for '%s' in import is not an integer: %#v", name, v)
		}
		*dest = i
	}

	if ret.Path == "" {
		return ret, errors.Errorf("No 'path' entry found in import: %#v", v)
	}

	if ret.Dest != "" && !filepath.IsAbs(ret.Dest) {
		return Import{}, errors.Errorf("'dest' path cannot be relative for: %#v", v)
	}

	if mode != -1 {
		m := fs.FileMode(mode)
		ret.Mode = &m
	}

	// Empty values are -1
	if ret.Uid != lib.UidEmpty && ret.Uid < 0 {
		return Import{}, errors.Errorf("'uid' (%d) cannot be negative: %v", ret.Uid, v)
	}
	if ret.Gid != lib.GidEmpty && ret.Gid < 0 {
		return Import{}, errors.Errorf("'gid' (%d) cannot be negative: %v", ret.Gid, v)
	}

	return ret, nil
}

// Custom UnmarshalYAML from string/map/slice of strings/slice of maps into Imports
func (im *Imports) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}
	if err := unmarshal(&data); err != nil {
		return err
	}

	imports, ok := data.([]interface{})
	if !ok {
		// "import: /path/to/file" is also supported (single import)
		imp, ok := data.(string)
		if !ok {
			return errors.Errorf("'imports' expected an array, found %s: %#v", reflect.TypeOf(data), data)
		}
		imports = []interface{}{imp}
	}
	// imports are a list of either strings or maps
	for _, v := range imports {
		imp, err := getImportFromInterface(v)
		if err != nil {
			return err
		}
		*im = append(*im, imp)
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
