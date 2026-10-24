package types

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/log"
)

// Logic for working with multiple StackerFiles
type StackerFiles map[string]*Stackerfile

// NewStackerFiles reads multiple Stackerfiles from a list of paths and applies substitutions
// It adds the Stackerfiles mentioned in the prerequisite paths to the results
func NewStackerFiles(paths []string, validateHash bool, substituteVars []string) (StackerFiles, error) {
	sfm := make(StackerFiles, len(paths))

	err := sfm.addStackerFiles(paths, validateHash, substituteVars, []string{})
	if err != nil {
		return nil, err
	}

	// now, make sure output layer names are unique
	names := map[string]string{}
	for path, sf := range sfm {
		for _, layerName := range sf.FileOrder {
			if otherFile, ok := names[layerName]; ok {
				return nil, errors.Errorf("duplicate layer name: both %s and %s have %s", otherFile, path, layerName)
			}

			names[layerName] = path
		}
	}

	return sfm, nil
}

// addStackerFiles reads the Stackerfiles from a list of paths, along with
// their prerequisites, and adds the ones that have not been read yet to sfm.
// parents is the chain of prerequisites that led to these paths, which is
// used to detect prerequisite cycles.
func (sfm StackerFiles) addStackerFiles(paths []string, validateHash bool, substituteVars []string,
	parents []string) error {
	for _, path := range paths {
		// Use the absolute path to make sure the entries are unique
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		for i, parent := range parents {
			if parent == absPath {
				cycle := append(append([]string{}, parents[i:]...), absPath)
				return errors.Errorf("prerequisite cycle detected: %s", strings.Join(cycle, " -> "))
			}
		}

		if _, ok := sfm[absPath]; ok {
			continue
		}

		log.Debugf("initializing stacker recipe: %s", path)

		// Read this stackerfile
		sf, err := NewStackerfile(path, validateHash, substituteVars)
		if err != nil {
			return err
		}
		sfm[absPath] = sf

		// Determine correct path of prerequisites
		prerequisites, err := sf.Prerequisites()
		if err != nil {
			return err
		}

		// Need to also add stackerfile dependencies of this stackerfile to the map of stackerfiles
		err = sfm.addStackerFiles(prerequisites, validateHash, substituteVars, append(parents, absPath))
		if err != nil {
			return err
		}
	}

	return nil
}

// LookupLayerDefinition searches for the Layer entry within the Stackerfiles
func (sfm StackerFiles) LookupLayerDefinition(name string) (Layer, bool) {
	// Search for the layer in all of the stackerfiles
	for _, sf := range sfm {
		l, found := sf.Get(name)
		if found {
			return l, true
		}
	}
	return Layer{}, false
}
