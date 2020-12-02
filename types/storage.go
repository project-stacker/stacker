package types

import (
	"github.com/opencontainers/umoci/oci/casext"
)

type Storage interface {
	// Name of this storage driver (e.g. "btrfs")
	Name() string

	// Create does the initial work to create a storage tag to be used
	// in later operations.
	Create(path string) error

	// SetupEmptyRootfs() sets up an empty rootfs for contents to be
	// written in (e.g. if it's a base tar file to be extracted).
	SetupEmptyRootfs(name string) error

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

	// UpdateFSMetadata updates the filesystem metadata (e.g. umoci's mtree
	// files, or anything else needed) for generating deltas. This is used
	// after e.g. a build is complete, but before the snapshot is
	// Finalize()d.
	UpdateFSMetadata(name string, path casext.DescriptorPath) error

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

	// Unpack is the thing that unpacks the specfied tag layer-bases OCI
	// cache into the specified "name" (working dir), whatever that means
	// for this storage.
	//
	// Unpack can do fancy things like using previously cached unpacks to
	// speed things up, etc.
	Unpack(tag, name string) error

	// Repack repacks the specified working dir into the specified OCI dir.
	Repack(name string, layerTypes []LayerType, sfm StackerFiles) error

	// GetLXCRootfsConfig returns the string that should be set as
	// lxc.rootfs.path in the LXC container's config.
	GetLXCRootfsConfig(name string) (string, error)

	// TarExtractLocation returns the location that a tar-based rootfs
	// should be extracted to
	TarExtractLocation(name string) string
}
