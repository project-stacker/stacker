// A basic overlay storage backend.
//
// Things still TODO:
//  1. implement GC (nobody really uses this, it seems people just clean and
//     rebuild, so...)
package overlay

import (
	"fmt"
	"os"
	"path"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"stackerbuild.io/stacker/pkg/types"
)

var _ types.Storage = &overlay{}

// canMountOverlay detects whether the current task can mount overlayfs
// successfully (some kernels (ubuntu) support unprivileged overlay mounts, and
// some do not).
func canMountOverlay() error {
	dir, err := os.MkdirTemp("", "stacker-overlay-mount-")
	if err != nil {
		return errors.Wrapf(err, "couldn't create overlay tmpdir")
	}
	defer os.RemoveAll(dir)

	// overlay doesn't work with one lowerdir... bleh
	lower1 := path.Join(dir, "lower1")
	err = os.Mkdir(lower1, 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create overlay lower dir")
	}

	lower2 := path.Join(dir, "lower2")
	err = os.Mkdir(lower2, 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create overlay lower dir")
	}

	mountpoint := path.Join(dir, "mountpoint")
	err = os.Mkdir(mountpoint, 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create overlay mountpoint dir")
	}

	opts := fmt.Sprintf("lowerdir=%s:%s", lower1, lower2)
	err = unix.Mount("overlay", mountpoint, "overlay", 0, opts)
	defer unix.Unmount(mountpoint, 0)
	if err != nil {
		return errors.Wrapf(err, "couldn't mount overlayfs")
	}
	return nil
}

func isOverlayfs(dir string) (bool, error) {
	fs := syscall.Statfs_t{}

	err := syscall.Statfs(dir, &fs)
	if err != nil {
		return false, errors.Wrapf(err, "failed to stat for overlayfs")
	}

	/* overlayfs superblock magic number */
	return fs.Type == 0x794c7630, nil
}

// canWriteWhiteouts detects whether the current task can write whiteouts. The
// upstream kernel as of v5.8 a3c751a50fe6 ("vfs: allow unprivileged whiteout
// creation") allows this as an unprivileged user.
func canWriteWhiteouts(config types.StackerConfig) error {
	// if the underlying filesystem is an overlay, we can't do this mknod
	// because it is explicitly forbidden in the kernel.
	isOverlay, err := isOverlayfs(config.RootFSDir)
	if err != nil {
		return err
	}

	if isOverlay {
		return errors.Errorf("can't create overlay whiteout on underlying overlayfs in %s", config.RootFSDir)
	}

	dir, err := os.MkdirTemp(config.RootFSDir, "stacker-overlay-whiteout-")
	if err != nil {
		return errors.Wrapf(err, "couldn't create overlay tmpdir")
	}
	defer os.RemoveAll(dir)

	err = unix.Mknod(path.Join(dir, "test"), syscall.S_IFCHR|0666, int(unix.Mkdev(0, 0)))
	if err != nil {
		if os.IsPermission(err) {
			return errors.Errorf("can't create overlay whiteouts (use a kernel >= 5.8)")
		}

		return errors.Wrapf(err, "couldn't create overlay whiteout")
	}

	return nil
}

func Check(config types.StackerConfig) error {
	err := canMountOverlay()
	if err != nil {
		return err
	}

	return canWriteWhiteouts(config)
}

type overlay struct {
	config types.StackerConfig
}

func NewOverlay(config types.StackerConfig) (types.Storage, error) {
	return &overlay{config}, nil
}

func (o *overlay) Name() string {
	return "overlay"
}

func (o *overlay) Create(source string) error {
	err := os.MkdirAll(path.Join(o.config.RootFSDir, source, "overlay"), 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create %s", source)
	}

	err = os.MkdirAll(path.Join(o.config.RootFSDir, source, "rootfs"), 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create %s", source)
	}

	return nil
}

func (o *overlay) SetupEmptyRootfs(name string) error {
	ovl := overlayMetadata{}
	return ovl.write(o.config, name)
}

func (o *overlay) snapshot(source string, target string) error {
	err := o.Create(target)
	if err != nil {
		return err
	}

	// now we need to replicate the overlay mount if it exists
	ovl, err := readOverlayMetadata(o.config, source)
	if err != nil {
		return err
	}

	ovl.BuiltLayers = append(ovl.BuiltLayers, source)

	return ovl.write(o.config, target)
}

func (o *overlay) Snapshot(source, target string) error {
	return o.snapshot(source, target)
}

func (o *overlay) Restore(source, target string) error {
	return o.snapshot(source, target)
}

func (o *overlay) Delete(thing string) error {
	return errors.Wrapf(os.RemoveAll(path.Join(o.config.RootFSDir, thing)), "couldn't delete %s", thing)
}

func (o *overlay) Exists(thing string) bool {
	_, err := os.Stat(path.Join(o.config.RootFSDir, thing))
	return err == nil
}

func (o *overlay) TemporaryWritableSnapshot(source string) (string, func(), error) {
	// should use create maybe?
	dir, err := os.MkdirTemp(o.config.RootFSDir, fmt.Sprintf("temp-snapshot-%s-", source))
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to create snapshot")
	}

	cleanup := func() {
		unix.Unmount(path.Join(dir, "rootfs"), 0)
		o.Delete(dir)
	}

	err = o.Snapshot(source, path.Base(dir))
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return path.Base(dir), cleanup, nil
}

func (o *overlay) Clean() error {
	return errors.Wrapf(os.RemoveAll(o.config.RootFSDir), "couldn't clean rootfs dir")
}

func (o *overlay) GC() error {
	// see note in pack.go about implementing this
	return errors.Errorf("todo")
}

func (o *overlay) GetLXCRootfsConfig(name string) (string, error) {
	ovl, err := readOverlayMetadata(o.config, name)
	if err != nil {
		return "", err
	}

	lxcRootfsString, err := ovl.lxcRootfsString(o.config, name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("overlay:%s,userxattr", lxcRootfsString), nil
}

func (o *overlay) TarExtractLocation(name string) string {
	return path.Join(o.config.RootFSDir, name, "overlay")
}

// Add overlay_dirs into overlay metadata so that later we can mount them in the lxc container
func (o *overlay) SetOverlayDirs(name string, overlayDirs []types.OverlayDir, layerTypes []types.LayerType) error {
	if len(overlayDirs) == 0 {
		return nil
	}
	// copy overlay_dirs contents into a temporary dir in roots
	err := copyOverlayDirs(name, overlayDirs, o.config.RootFSDir)
	if err != nil {
		return err
	}
	// generate layers from above overlay_dirs
	err = generateOverlayDirsLayers(name, layerTypes, overlayDirs, o.config)
	if err != nil {
		return err
	}

	return nil
}
