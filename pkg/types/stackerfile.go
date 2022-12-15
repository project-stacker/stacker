package types

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"stackerbuild.io/stacker/pkg/log"
)

type BuildConfig struct {
	Prerequisites []string `yaml:"prerequisites"`
}

type Stackerfile struct {
	// AfterSubstitutions is the contents of the stacker file after
	// substitutions (i.e., the content that is actually used by stacker).
	AfterSubstitutions string

	// internal is the actual representation of the stackerfile as a map.
	internal map[string]Layer

	// FileOrder is the order of elements as they appear in the stackerfile.
	FileOrder []string

	// configuration specific for this specific build
	buildConfig *BuildConfig

	// path to stackerfile
	path string

	// directory relative to which the stackerfile content is referenced
	ReferenceDirectory string
}

func (sf *Stackerfile) Get(name string) (Layer, bool) {
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
func NewStackerfile(stackerfile string, validateHash bool, substitutions []string) (*Stackerfile, error) {
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
		raw, err = os.ReadFile(stackerfile)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't read stacker file")
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

		raw, err = io.ReadAll(resp.Body)
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

	sf.internal, err = parseLayers(sf.ReferenceDirectory, lms, validateHash)
	if err != nil {
		return nil, err
	}

	return &sf, err
}

func (s *Stackerfile) addPrerequisites(processed map[string]bool, sfm StackerFiles) error {
	for _, prereq := range s.buildConfig.Prerequisites {
		absPrereq := prereq
		if !filepath.IsAbs(absPrereq) {
			absPrereq = filepath.Join(filepath.Dir(s.path), prereq)
		}
		prereqFile, ok := sfm[absPrereq]
		if !ok {
			processedPrereqs := []string{}
			for prereqPath := range sfm {
				processedPrereqs = append(processedPrereqs, prereqPath)
			}
			log.Debugf("processed prereqs %v", processedPrereqs)
			return errors.Errorf("couldn't find prerequisite %s", absPrereq)
		}

		// prerequisites are processed beforehand
		for thing := range prereqFile.internal {
			processed[thing] = true
		}

		err := prereqFile.addPrerequisites(processed, sfm)
		if err != nil {
			return err
		}
	}
	return nil
}

// DependencyOrder provides the list of layer names from a stackerfile in the
// order in which they should be built so all dependencies are satisfied.
func (s *Stackerfile) DependencyOrder(sfm StackerFiles) ([]string, error) {
	ret := []string{}
	processed := map[string]bool{}

	err := s.addPrerequisites(processed, sfm)
	if err != nil {
		return nil, err
	}

	getUnprocessedStackerImports := func(layer Layer) ([]string, error) {
		unprocessed := []string{}

		// Determine if the layer has stacker:// imports from another
		// layer which has not been processed
		for _, imp := range layer.Imports {
			url, err := NewDockerishUrl(imp.Path)
			if err != nil {
				return nil, err
			}

			if url.Scheme != "stacker" {
				continue
			}

			_, ok := processed[url.Host]
			if !ok {
				unprocessed = append(unprocessed, imp.Path)
			}
		}

		if layer.From.Type == TarLayer {
			url, err := NewDockerishUrl(layer.From.Url)
			if err != nil {
				return nil, err
			}

			if url.Scheme == "stacker" {
				_, ok := processed[url.Host]
				if !ok {
					unprocessed = append(unprocessed, layer.From.Url)
				}
			}
		}

		return unprocessed, nil

	}

	for i := 0; i < s.Len(); i++ {
		for _, name := range s.FileOrder {
			_, ok := processed[name]
			if ok {
				continue
			}

			layer := s.internal[name]

			if layer.From.Type == "" {
				return nil, errors.Errorf("invalid layer: no base (from directive)")
			}

			// Determine if the layer uses a previously processed layer as base
			_, baseTagProcessed := processed[layer.From.Tag]

			unprocessedImports, err := getUnprocessedStackerImports(layer)
			if err != nil {
				return nil, err
			}

			if len(unprocessedImports) == 0 && (layer.From.Type != BuiltLayer || baseTagProcessed) {
				// None of the imports using stacker:// are referencing unprocessed layers,
				// and in case the base layer is type build we have already processed it
				ret = append(ret, name)
				processed[name] = true
			}
		}
	}

	if len(ret) != s.Len() {
		for _, name := range s.FileOrder {
			layer := s.internal[name]

			_, ok := processed[name]
			if ok {
				continue
			}

			unprocessedDeps, err := getUnprocessedStackerImports(layer)
			if err != nil {
				return nil, err
			}

			_, baseTagProcessed := processed[layer.From.Tag]
			if layer.From.Type == BuiltLayer && !baseTagProcessed {
				unprocessedDeps = append(unprocessedDeps, fmt.Sprintf("base layer %s", layer.From.Tag))
			}

			log.Infof("couldn't find dependencies for %s: %s", name, strings.Join(unprocessedDeps, ", "))
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
