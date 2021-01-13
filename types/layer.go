package types

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/pkg/errors"
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

type Layer struct {
	From               *ImageSource      `yaml:"from"`
	Import             interface{}       `yaml:"import"`
	Run                interface{}       `yaml:"run"`
	Cmd                interface{}       `yaml:"cmd"`
	Entrypoint         interface{}       `yaml:"entrypoint"`
	FullCommand        interface{}       `yaml:"full_command"`
	BuildEnvPt         []string          `yaml:"build_env_passthrough"`
	BuildEnv           map[string]string `yaml:"build_env"`
	Environment        map[string]string `yaml:"environment"`
	Volumes            []string          `yaml:"volumes"`
	Labels             map[string]string `yaml:"labels"`
	GenerateLabels     interface{}       `yaml:"generate_labels"`
	WorkingDir         string            `yaml:"working_dir"`
	BuildOnly          bool              `yaml:"build_only"`
	Binds              interface{}       `yaml:"binds"`
	RuntimeUser        string            `yaml:"runtime_user"`
	referenceDirectory string            // Location of the directory where the layer is defined
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
	rawImports, err := l.getStringOrStringSlice(l.Import, func(s string) ([]string, error) {
		return strings.Split(s, "\n"), nil
	})
	if err != nil {
		return nil, err
	}

	var absImports []string
	for _, rawImport := range rawImports {
		absImport, err := l.getAbsPath(rawImport)
		if err != nil {
			return nil, err
		}
		absImports = append(absImports, absImport)
	}
	return absImports, nil
}

func (l *Layer) ParseBinds() (map[string]string, error) {
	rawBinds, err := l.getStringOrStringSlice(l.Binds, func(s string) ([]string, error) {
		return []string{s}, nil
	})
	if err != nil {
		return nil, err
	}

	absBinds := make(map[string]string, len(rawBinds))
	for _, bind := range rawBinds {
		parts := strings.Split(bind, "->")
		if len(parts) != 1 && len(parts) != 2 {
			return nil, errors.Errorf("invalid bind mount %s", bind)
		}

		source := strings.TrimSpace(parts[0])
		target := source

		absSource, err := l.getAbsPath(source)
		if err != nil {
			return nil, err
		}

		if len(parts) == 2 {
			target = strings.TrimSpace(parts[1])
		}

		absBinds[absSource] = target
	}

	return absBinds, nil

}

func (l *Layer) ParseRun() ([]string, error) {
	return l.getStringOrStringSlice(l.Run, func(s string) ([]string, error) {
		return []string{s}, nil
	})
}

func (l *Layer) ParseGenerateLabels() ([]string, error) {
	return l.getStringOrStringSlice(l.GenerateLabels, func(s string) ([]string, error) {
		return []string{s}, nil
	})
}

func (l *Layer) getAbsPath(path string) (string, error) {
	parsedPath, err := NewDockerishUrl(path)
	if err != nil {
		return "", err
	}

	if parsedPath.Scheme != "" || filepath.IsAbs(path) {
		// Path is already absolute or is an URL, return it
		return path, nil
	} else {
		// If path is relative we need to add it to the directory where this layer is found
		return filepath.Abs(filepath.Join(l.referenceDirectory, path))
	}
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
				return nil, errors.Errorf("unknown run array type: %T", i)
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

	return nil, errors.Errorf("unknown directive type: %T", l.Run)
}

var (
	layerFields []string
)

func init() {
	layerFields = []string{}
	layerType := reflect.TypeOf(Layer{})
	for i := 0; i < layerType.NumField(); i++ {
		tag := layerType.Field(i).Tag.Get("yaml")
		layerFields = append(layerFields, tag)
	}
}
