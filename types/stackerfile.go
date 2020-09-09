package types

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anuvu/stacker/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type BuildConfig struct {
	Prerequisites []string `yaml:"prerequisites"`
}

type Stackerfile struct {
	// AfterSubstitutions is the contents of the stacker file after
	// substitutions (i.e., the content that is actually used by stacker).
	AfterSubstitutions string

	// internal is the actual representation of the stackerfile as a map.
	internal map[string]*Layer

	// FileOrder is the order of elements as they appear in the stackerfile.
	FileOrder []string

	// configuration specific for this specific build
	buildConfig *BuildConfig

	// path to stackerfile
	path string

	// directory relative to which the stackerfile content is referenced
	ReferenceDirectory string
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

func substitute(content string, substitutions []string) (string, error) {
	for _, subst := range substitutions {
		membs := strings.SplitN(subst, "=", 2)
		if len(membs) != 2 {
			return "", errors.Errorf("invalid substition %s", subst)
		}

		from := fmt.Sprintf("$%s", membs[0])
		to := membs[1]

		log.Debugf("substituting %s to %s", from, to)

		content = strings.Replace(content, from, to, -1)

		re, err := regexp.Compile(fmt.Sprintf(`\$\{\{%s(:[^\}]*)?\}\}`, membs[0]))
		if err != nil {
			return "", err
		}

		content = re.ReplaceAllString(content, to)
	}

	// now, anything that's left we can just use its value
	re := regexp.MustCompile(`\$\{\{[^\}]*\}\}`)
	for {
		indexes := re.FindAllStringIndex(content, -1)
		if len(indexes) == 0 {
			break
		}

		idx := indexes[0]

		// get content without ${{}}
		variable := content[idx[0]+3 : idx[1]-2]

		membs := strings.SplitN(variable, ":", 2)
		if len(membs) != 2 {
			return "", errors.Errorf("no value for substitution %s", variable)
		}

		buf := bytes.NewBufferString(content[:idx[0]])
		_, err := buf.WriteString(membs[1])
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
	var err error

	sf := Stackerfile{}
	sf.path = stackerfile

	// Use working directory as default folder relative to which files
	// in the stacker yaml will be searched for
	sf.ReferenceDirectory, err = os.Getwd()
	if err != nil {
		return nil, err
	}

	url, err := NewDockerishUrl(stackerfile)
	if err != nil {
		return nil, err
	}

	var raw []byte
	if url.Scheme == "" {
		raw, err = ioutil.ReadFile(stackerfile)
		if err != nil {
			return nil, err
		}

		// Make sure we use the absolute path to the Stackerfile
		sf.path, err = filepath.Abs(stackerfile)
		if err != nil {
			return nil, err
		}

		// This file is on the disk, use its parent directory
		sf.ReferenceDirectory = filepath.Dir(sf.path)

	} else {
		resp, err := http.Get(stackerfile)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, errors.Errorf("stackerfile: couldn't download %s: %s", stackerfile, resp.Status)
		}

		raw, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		// There's no need to update the reference directory of the stackerfile
		// Continue to use the working directory
	}

	content, err := substitute(string(raw), substitutions)
	if err != nil {
		return nil, err
	}

	sf.AfterSubstitutions = content

	// Parse the first time to validate the format/content
	ms := yaml.MapSlice{}
	if err := yaml.Unmarshal([]byte(content), &ms); err != nil {
		return nil, errors.Wrapf(err, "couldn't parse stacker file %s", stackerfile)
	}

	// Determine the layers in the stacker.yaml, their order and the list of prerequisite files
	sf.FileOrder = []string{}      // Order of layers
	sf.buildConfig = &BuildConfig{ // Stacker build configuration
		Prerequisites: []string{},
	}
	lms := yaml.MapSlice{} // Actual list of layers excluding the config directive
	for _, e := range ms {
		keyName, ok := e.Key.(string)
		if !ok {
			return nil, errors.Errorf("stackerfile: cannot cast %v to string", e.Key)
		}

		if "config" == keyName {
			stackerConfigContent, err := yaml.Marshal(e.Value)
			if err != nil {
				return nil, err
			}
			if err = yaml.Unmarshal(stackerConfigContent, &sf.buildConfig); err != nil {
				msg := fmt.Sprintf("stackerfile: cannot interpret 'config' value, "+
					"note the 'config' section in the stackerfile cannot contain a layer definition %v", e.Value)
				return nil, errors.New(msg)
			}
		} else {
			sf.FileOrder = append(sf.FileOrder, e.Key.(string))
			lms = append(lms, e)
		}
	}

	// Now, let's make sure that all the things people supplied in the layers are
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
		}
	}

	// Marshall only the layers so we can unmarshal them in the right data structure later
	layersContent, err := yaml.Marshal(lms)
	if err != nil {
		return nil, err
	}

	// Unmarshal to save the data in the right structure to enable further processing
	if err := yaml.Unmarshal(layersContent, &sf.internal); err != nil {
		return nil, err
	}

	for name, layer := range sf.internal {
		// Validate field values
		switch layer.From.Type {
		case BuiltLayer:
			if len(layer.From.Tag) == 0 {
				return nil, errors.Errorf("%s: from tag cannot be empty for image type 'built'", name)
			}
		}

		// Set the directory with the location where the layer was defined
		layer.referenceDirectory = sf.ReferenceDirectory
	}

	return &sf, err
}

// DependencyOrder provides the list of layer names from a stackerfile
// the current order to be built, note this method does not reorder the layers,
// but it does validate they are specified in an order which makes sense
func (s *Stackerfile) DependencyOrder(sfm StackerFiles) ([]string, error) {
	ret := []string{}
	processed := map[string]bool{}

	for _, prereq := range s.buildConfig.Prerequisites {
		absPrereq := filepath.Join(filepath.Dir(s.path), prereq)
		prereqFile, ok := sfm[absPrereq]
		if !ok {
			return nil, errors.Errorf("couldn't find prerequisite %s", prereq)
		}

		// prerequisites are processed beforehand
		for thing := range prereqFile.internal {
			processed[thing] = true
		}
	}

	for i := 0; i < s.Len(); i++ {
		for _, name := range s.FileOrder {
			_, ok := processed[name]
			if ok {
				continue
			}

			layer := s.internal[name]

			if layer.From == nil {
				return nil, errors.Errorf("invalid layer: no base (from directive)")
			}

			// Determine if the layer uses a previously processed layer as base
			_, baseTagProcessed := processed[layer.From.Tag]

			imports, err := layer.ParseImport()
			if err != nil {
				return nil, err
			}

			// Determine if the layer has stacker:// imports from another
			// layer which has not been processed
			allStackerImportsProcessed := true
			for _, imp := range imports {
				url, err := NewDockerishUrl(imp)
				if err != nil {
					return nil, err
				}

				if url.Scheme != "stacker" {
					continue
				}

				_, ok := processed[url.Host]
				if !ok {
					allStackerImportsProcessed = false
					break
				}
			}

			if allStackerImportsProcessed && (layer.From.Type != BuiltLayer || baseTagProcessed) {
				// None of the imports using stacker:// are referencing unprocessed layers,
				// and in case the base layer is type build we have already processed it
				ret = append(ret, name)
				processed[name] = true
			}
		}
	}

	if len(ret) != s.Len() {
		for _, name := range s.FileOrder {
			_, ok := processed[name]
			if !ok {
				log.Infof("couldn't find dependencies for %s", name)
			}
		}
		return nil, errors.Errorf("couldn't resolve some dependencies")
	}

	return ret, nil
}

// Prerequisites provides the absolute paths to the Stackerfiles which are dependencies
// for building this Stackerfile
func (sf *Stackerfile) Prerequisites() ([]string, error) {
	// Cleanup paths in the prerequisites
	var prerequisitePaths []string
	for _, prerequisitePath := range sf.buildConfig.Prerequisites {
		parsedPath, err := NewDockerishUrl(prerequisitePath)
		if err != nil {
			return nil, err
		}
		if parsedPath.Scheme != "" || filepath.IsAbs(prerequisitePath) {
			// Path is already absolute or is an URL, return it
			prerequisitePaths = append(prerequisitePaths, prerequisitePath)
		} else {
			// If path is relative we need to add it to the path to this stackerfile
			absPath, err := filepath.Abs(filepath.Join(sf.ReferenceDirectory, prerequisitePath))
			if err != nil {
				return nil, err
			}
			prerequisitePaths = append(prerequisitePaths, absPath)
		}
	}
	return prerequisitePaths, nil
}
