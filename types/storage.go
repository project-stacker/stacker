package types

type Storage interface {
	// Name of this storage driver (e.g. "btrfs")
	Name() string

	// Create does the initial work to create a storage tag to be used
	// in later operations.
	Create(path string) error

	// Snapshot "copies" (maybe in a fs-specific fast way) one tag to
	// another; snapshots should be readonly or not generally modifiable.
	Snapshot(source string, target string) error

	// Restore is like snapshot (in fact, the implementations may be the
	// same), but marks the result as writable.
	Restore(source string, target string) error

	// Delete a storage tag.
	Delete(path string) error

	// Test if a storage tag exists.
	Exists(thing string) bool

	// Unmount anything that this storage driver has mounted during
	// operation (in preparation for stacker to exit). No need to delete
	// anything, though.
	Detach() error

	// Finalize should seal the tag so it can no longer be modified
	// (although it will be used later during Repack(), but in a read only
	// sense).
	Finalize(thing string) error

	// Create a temporary writable snapshot of the source, returning the
	// snapshot's tag and a cleanup function.
	TemporaryWritableSnapshot(source string) (string, func(), error)

	// Clean the storage: do unmounting, delete all caches/tags, etc.
	Clean() error

	// GC any storage that's no longer relevant for the layers in the
	// layer-bases cache or output directory (note that this implies a GC
	// of those OCI dirs as well).
	GC() error

	// Unpack is the thing that unpacks the specfied tag from the specified
	// ociDir into the specified "name" (working dir), whatever that means
	// for this storage.
	//
	// Unpack can do fancy things like using previously cached unpacks to
	// speed things up, etc.
	Unpack(ociDir, tag, name string) error

	// Repack repacks the specified working dir into the specified OCI dir.
	//
	// TODO: make layerType an enum :)
	Repack(ociDir, name, layerType string) error
}
