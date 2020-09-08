// A basic overlay storage backend.
//
//
// Things still TODO:
//
// 3. support squashfs
// 4. implement GC (nobody really uses this, it seems people just clean and
//    rebuild, so...)
package overlay

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/mount"
	"github.com/anuvu/stacker/types"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var _ types.Storage = &overlay{}

// canMountOverlay detects whether the current task can mount overlayfs
// successfully (some kernels (ubuntu) support unprivileged overlay mounts, and
// some do not).
func canMountOverlay() (bool, error) {
	dir, err := ioutil.TempDir("", "stacker-overlay-mount-")
	if err != nil {
		return false, errors.Wrapf(err, "couldn't create overlay tmpdir")
	}
	defer os.RemoveAll(dir)

	// overlay doesn't work with one lowerdir... bleh
	lower1 := path.Join(dir, "lower1")
	err = os.Mkdir(lower1, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't create overlay lower dir")
	}

	lower2 := path.Join(dir, "lower2")
	err = os.Mkdir(lower2, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't create overlay lower dir")
	}

	mountpoint := path.Join(dir, "mountpoint")
	err = os.Mkdir(mountpoint, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't create overlay mountpoint dir")
	}

	opts := fmt.Sprintf("lowerdir=%s:%s", lower1, lower2)
	err = unix.Mount("overlay", mountpoint, "overlay", 0, opts)
	defer unix.Unmount(mountpoint, 0)
	if err != nil {
		log.Debugf("can't mount overlayfs: %v", err)
	}
	return err == nil, nil
}

// canWriteWhiteouts detects whether the current task can write whiteouts. The
// upstream kernel as of v5.8 a3c751a50fe6 ("vfs: allow unprivileged whiteout
// creation") allows this as an unprivileged user.
func canWriteWhiteouts() (bool, error) {
	dir, err := ioutil.TempDir("", "stacker-overlay-whiteout-")
	if err != nil {
		return false, errors.Wrapf(err, "couldn't create overlay tmpdir")
	}
	defer os.RemoveAll(dir)

	err = unix.Mknod(path.Join(dir, "test"), syscall.S_IFCHR|0666, int(unix.Mkdev(0, 0)))
	if err != nil {
		if os.IsPermission(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func CanDoOverlay() (bool, error) {
	canMount, err := canMountOverlay()
	if err != nil {
		return false, err
	}
	if !canMount {
		return false, nil
	}

	return canWriteWhiteouts()
}

type overlay struct {
	config types.StackerConfig
}

func NewOverlay(config types.StackerConfig) types.Storage {
	// TODO: we should go through all the non-sha things in
	// config.RootFSDir and mount anything that looks like it might be
	// used (Deatch() will unmount it all).
	return &overlay{config}
}

func (o *overlay) Name() string {
	return "overlay"
}

func (o *overlay) Create(source string) error {
	err := os.MkdirAll(path.Join(o.config.RootFSDir, source, "overlay"), 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't create %s", source)
	}

	err = os.MkdirAll(path.Join(o.config.RootFSDir, source, "work"), 0755)
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
	err := ovl.write(o.config, name)
	if err != nil {
		return err
	}

	return ovl.mount(o.config, name)
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

	err = ovl.write(o.config, target)
	if err != nil {
		return err
	}

	return ovl.mount(o.config, target)
}

func (o *overlay) Snapshot(source, target string) error {
	return o.snapshot(source, target)
}

func (o *overlay) Restore(source, target string) error {
	return o.snapshot(source, target)
}

func (o *overlay) Delete(thing string) error {
	rootfs := path.Join(o.config.RootFSDir, thing, "rootfs")
	mounted, err := mount.IsMountpoint(rootfs)
	if err != nil {
		return err
	}

	if mounted {
		err := unix.Unmount(rootfs, 0)
		if err != nil {
			return errors.Wrapf(err, "couldn't unmount %s", thing)
		}
	}
	return errors.Wrapf(os.RemoveAll(path.Join(o.config.RootFSDir, thing)), "couldn't delete %s", thing)
}

func (o *overlay) Exists(thing string) bool {
	_, err := os.Stat(path.Join(o.config.RootFSDir, thing))
	return err == nil
}

func (o *overlay) Detach() error {
	mounts, err := mount.ParseMounts("/proc/self/mountinfo")
	if err != nil {
		return err
	}

	for _, mount := range mounts {
		if !strings.HasPrefix(mount.Target, o.config.RootFSDir) {
			continue
		}

		err = unix.Unmount(mount.Target, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to unmount %s", mount.Target)
		}
	}

	return nil
}

func (o *overlay) UpdateFSMetadata(name string, path casext.DescriptorPath) error {
	// no-op; we get our layer contents by just looking at the contents of
	// the upperdir
	return nil
}

func (o *overlay) Finalize(thing string) error {
	return nil
}

func (o *overlay) TemporaryWritableSnapshot(source string) (string, func(), error) {
	// should use create maybe?
	dir, err := ioutil.TempDir(o.config.RootFSDir, fmt.Sprintf("temp-snapshot-%s-", source))
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
	return errors.Errorf("todo")
}
