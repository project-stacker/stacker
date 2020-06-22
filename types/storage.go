package types

type Storage interface {
	Name() string
	Create(path string) error
	Snapshot(source string, target string) error
	Restore(source string, target string) error
	Delete(path string) error
	Detach() error
	Exists(thing string) bool
	MarkReadOnly(thing string) error
	TemporaryWritableSnapshot(source string) (string, func(), error)
	Clean() error

	// GC any storage that's no longer relevant for the layers in the
	// layer-bases cache or output directory (note that this implies a GC
	// of those OCI dirs as well).
	GC() error
}
