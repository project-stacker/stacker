package types

import (
	"path/filepath"

	"github.com/anuvu/stacker/log"
)

// Logic for working with multiple StackerFiles
type StackerFiles map[string]*Stackerfile

// NewStackerFiles reads multiple Stackerfiles from a list of paths and applies substitutions
// It adds the Stackerfiles mentioned in the prerequisite paths to the results
func NewStackerFiles(paths []string, substituteVars []string) (StackerFiles, error) {
	sfm := make(map[string]*Stackerfile, len(paths))

	// Iterate over list of paths to stackerfiles
	for _, path := range paths {
		log.Debugf("initializing stacker recipe: %s", path)

		// Read this stackerfile
		sf, err := NewStackerfile(path, substituteVars)
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
		depStackerFiles, err := NewStackerFiles(prerequisites, substituteVars)
		if err != nil {
			return nil, err
		}
		for depPath, depStackerFile := range depStackerFiles {
			sfm[depPath] = depStackerFile
		}
	}

	return sfm, nil
}

// LookupLayerDefinition searches for the Layer entry within the Stackerfiles
func (sfm StackerFiles) LookupLayerDefinition(name string) (*Layer, bool) {
	// Search for the layer in all of the stackerfiles
	for _, sf := range sfm {
		l, found := sf.Get(name)
		if found {
			return l, true
		}
	}
	return nil, false
}
