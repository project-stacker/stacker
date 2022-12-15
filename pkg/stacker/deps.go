package stacker

import (
	"sort"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/types"
)

// StackerDepsDAG processes the dependencies between different stacker recipes
type StackerFilesDAG struct {
	dag lib.Graph
}

// NewStackerDepsDAG properly initializes a StackerDepsProcessor
func NewStackerFilesDAG(sfMap types.StackerFiles) (*StackerFilesDAG, error) {
	dag := lib.NewDAG()

	// The DAG.Sort() method uses Topological sort which for acyclical graphs
	// will return an order dependent upon which node it starts.  To ensure
	// we build the same Graph, sort the list of input files so we get the
	// same starting Node for DAG.Sort() resulting in a consistent build order.
	keys := make([]string, 0, len(sfMap))
	for k := range sfMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Add vertices to dag
	for _, path := range keys {
		sf := sfMap[path]
		// Add a vertex for every StackerFile object
		err := dag.AddVertex(path, sf)
		if err != nil {
			return nil, err
		}
	}

	// Update the dependencies in the dag
	for path, sf := range sfMap {
		prerequisites, err := sf.Prerequisites()
		if err != nil {
			return nil, err
		}

		for _, depPath := range prerequisites {
			err := dag.AddDependencies(path, depPath)
			if err != nil {
				return nil, err
			}
		}
	}

	p := StackerFilesDAG{
		dag: dag,
	}
	return &p, nil
}

func (d *StackerFilesDAG) GetStackerFile(path string) *types.Stackerfile {
	value := d.dag.GetValue(path)
	return value.(*types.Stackerfile)
}

// Sort provides a serial build order for the stacker files
func (d *StackerFilesDAG) Sort() []string {
	var order []string

	// Use dag.Sort() to ensure we always process targets in order of their dependencies
	for _, i := range d.dag.Sort() {
		path := i.Key.(string)
		order = append(order, path)
	}

	return order
}
