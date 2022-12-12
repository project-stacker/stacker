package mtree

import (
	"github.com/opencontainers/umoci/pkg/mtreefilter"
)

var (
	// Stacker does a mkdir /stacker for bind mounting in imports and such.
	// Unfortunately, this causes the mtime on the directory to be changed,
	// and go-mtree picks that upas a diff and always generates it. Let's
	// mask this out. This of course prevents stuff like `chmod 0444 /` or
	// similar, but that's not a very common use case.
	LayerGenerationIgnoreRoot mtreefilter.FilterFunc = func(path string) bool {
		// the paths are supplied relative to the filter dir, so '.' is root.
		return path != "."
	}
)
