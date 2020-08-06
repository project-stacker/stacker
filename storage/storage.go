// common code used by storage backends
package storage

import (
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
)

// FindFirstBaseInOutput finds the highest base in the dependency tree that is
// present in the output (i.e. it skips build-only layers).
func FindFirstBaseInOutput(name string, sfm types.StackerFiles) (string, *types.Layer, error) {
	// We need to copy any base OCI layers to the output dir, since they
	// may not have been copied before and the final `umoci repack` expects
	// them to be there.
	base, ok := sfm.LookupLayerDefinition(name)
	if !ok {
		return "", nil, errors.Errorf("couldn't find layer %s", name)
	}
	baseTag := name
	var err error

	// first, go all the way to the first layer that's not a built type
	for {
		if base.From.Type != types.BuiltLayer {
			break
		}

		baseTag, err = base.From.ParseTag()
		if err != nil {
			return "", nil, err
		}

		base, ok = sfm.LookupLayerDefinition(baseTag)
		if !ok {
			return "", nil, errors.Errorf("missing base layer: %s?", baseTag)
		}

		// if the base was emitted to the output, return that
		if !base.BuildOnly {
			return baseTag, base, nil
		}
	}

	// if this is from something in the OCI cache, we can use that
	if types.IsContainersImageLayer(base.From.Type) {
		return baseTag, base, nil
	}

	// otherwise, we didn't find anything
	return "", nil, nil
}
