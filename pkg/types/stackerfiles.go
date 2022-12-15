package types

import (
	"path/filepath"

	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/log"
)

// Logic for working with multiple StackerFiles
type StackerFiles map[string]*Stackerfile

// NewStackerFiles reads multiple Stackerfiles from a list of paths and applies substitutions
// It adds the Stackerfiles mentioned in the prerequisite paths to the results
func NewStackerFiles(paths []string, validateHash bool, substituteVars []string) (StackerFiles, error) {
	sfm := make(map[string]*Stackerfile, len(paths))

	// Iterate over list of paths to stackerfiles
	for _, path := range paths {
		log.Debugf("initializing stacker recipe: %s", path)

		// Read this stackerfile
		sf, err := NewStackerfile(path, validateHash, substituteVars)
		if err != nil {
			return nil, err
		}

		// Add using absolute path to make sure the entries are unique
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if _, ok := sfm[absPath]; !ok {
			sfm[absPath] = sf
		}

		// Determine correct path of prerequisites
		prerequisites, err := sf.Prerequisites()
		if err != nil {
			return nil, err
		}

		// Need to also add stackerfile dependencies of this stackerfile to the map of stackerfiles
		depStackerFiles, err := NewStackerFiles(prerequisites, validateHash, substituteVars)
		if err != nil {
			return nil, err
		}
		for depPath, depStackerFile := range depStackerFiles {
			sfm[depPath] = depStackerFile
		}
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
