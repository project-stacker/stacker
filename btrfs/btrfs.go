package btrfs

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/mount"
	"github.com/anuvu/stacker/types"
	"github.com/freddierice/go-losetup"
	"github.com/lxc/lxd/shared"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func DetectBtrfs(p string) (bool, error) {
	fs := syscall.Statfs_t{}

	err := syscall.Statfs(p, &fs)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't stat to detect btrfs")
	}

	/* btrfs superblock magic number */
	return fs.Type == 0x9123683E, nil
}

func NewLoopback(c types.StackerConfig) (types.Storage, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(c.StackerDir, 0755); err != nil {
		return nil, err
	}

	loopback := path.Join(c.StackerDir, "btrfs.loop")
	size := 100 * 1024 * 1024 * 1024
	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		return nil, err
	}

	gid, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		return nil, err
	}

	err = MakeLoopbackBtrfs(loopback, int64(size), uid, gid, c.RootFSDir)
	if err != nil {
		return nil, err
	}
	return &btrfs{c: c, needsUmount: true}, nil
}

func NewExisting(c types.StackerConfig) types.Storage {
	return &btrfs{c: c}
}

type btrfs struct {
	c           types.StackerConfig
	needsUmount bool
}

func (b *btrfs) Name() string {
	return "btrfs"
}

func (b *btrfs) sync(subvol string) error {
	p := path.Join(b.c.RootFSDir, subvol)
	fd, err := unix.Open(p, unix.O_RDONLY, 0)
	if err != nil {
		return errors.Wrapf(err, "couldn't open %s to sync", p)
	}
	defer unix.Close(fd)
	return errors.Wrapf(unix.Syncfs(fd), "couldn't sync fs at %s", subvol)
}

func (b *btrfs) Create(source string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"create",
		path.Join(b.c.RootFSDir, source)).CombinedOutput()
	if err != nil {
		return errors.Errorf("btrfs create: %s: %s", err, output)
	}

	return nil
}

func (b *btrfs) SetupEmptyRootfs(name string) error {
	return errors.Wrapf(os.Mkdir(path.Join(b.c.RootFSDir, name, "rootfs"), 0755), "couldn't init empty rootfs")
}

func (b *btrfs) Snapshot(source string, target string) error {
	if err := b.sync(source); err != nil {
		return err
	}

	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"snapshot",
		"-r",
		path.Join(b.c.RootFSDir, source),
		path.Join(b.c.RootFSDir, target)).CombinedOutput()
	if err != nil {
		return errors.Errorf("btrfs snapshot %s to %s: %s: %s", source, target, err, output)
	}

	return nil
}

func (b *btrfs) Restore(source string, target string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"snapshot",
		path.Join(b.c.RootFSDir, source),
		path.Join(b.c.RootFSDir, target)).CombinedOutput()
	if err != nil {
		return errors.Errorf("btrfs restore: %s: %s", err, output)
	}

	// Since we create snapshots as readonly above, we must re-mark them
	// writable here.
	output, err = exec.Command(
		"btrfs",
		"property",
		"set",
		"-ts",
		path.Join(b.c.RootFSDir, target),
		"ro",
		"false").CombinedOutput()
	if err != nil {
		return errors.Errorf("btrfs mark writable: %s: %s", err, output)
	}

	return nil
}

func (b *btrfs) UpdateFSMetadata(name string, newPath casext.DescriptorPath) error {
	rootPath := path.Join(b.c.RootFSDir, name)
	newName := strings.Replace(newPath.Descriptor().Digest.String(), ":", "_", 1) + ".mtree"

	infos, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return err
	}

	for _, fi := range infos {
		if !strings.HasSuffix(fi.Name(), ".mtree") {
			continue
		}

		err = os.Rename(path.Join(rootPath, fi.Name()), path.Join(rootPath, newName))
		if err != nil {
			return errors.Wrapf(err, "couldn't update mtree name")
		}
	}

	return umoci.WriteBundleMeta(rootPath, umoci.Meta{
		Version: umoci.MetaVersion,
		From:    newPath,
	})
}

func (b *btrfs) Finalize(thing string) error {
	if err := b.sync(thing); err != nil {
		return err
	}

	output, err := exec.Command(
		"btrfs",
		"property",
		"set",
		"-ts",
		path.Join(b.c.RootFSDir, thing),
		"ro",
		"true").CombinedOutput()
	if err != nil {
		return errors.Errorf("btrfs mark readonly: %s: %s", err, output)
	}
	return nil
}

// isBtrfsSubVolume returns true if the given Path is a btrfs subvolume else
// false.
func isBtrfsSubVolume(subvolPath string) (bool, error) {
	fs := syscall.Stat_t{}
	err := syscall.Lstat(subvolPath, &fs)
	if err != nil {
		return false, errors.Wrapf(err, "failed testing %s for subvol", subvolPath)
	}

	// Check if BTRFS_FIRST_FREE_OBJECTID
	if fs.Ino != 256 {
		return false, nil
	}

	// btrfs roots have the same inode as above, but they are not
	// subvolumes (and we can't delete them) so exlcude the path if it is a
	// mountpoint.
	mountpoint, err := mount.IsMountpoint(subvolPath)
	if err != nil {
		return false, err
	}

	if mountpoint {
		return false, nil
	}

	return true, nil
}

func btrfsSubVolumesGet(path string) ([]string, error) {
	result := []string{}

	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// Unprivileged users can't get to fs internals
	err := filepath.Walk(path, func(fpath string, fi os.FileInfo, err error) error {
		// Skip walk errors
		if err != nil {
			return nil
		}

		// Subvolumes can only be directories
		if !fi.IsDir() {
			return nil
		}

		// Check if a btrfs subvolume
		isSubvol, err := isBtrfsSubVolume(fpath)
		if err != nil {
			return err
		}

		if isSubvol {
			result = append(result, strings.TrimPrefix(fpath, path))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func btrfsSubVolumesDelete(root string) error {
	subvols, err := btrfsSubVolumesGet(root)
	if err != nil {
		return err
	}

	subvolsReversed := make([]string, len(subvols))
	copy(subvolsReversed, subvols)

	sort.Sort(sort.StringSlice(subvols))
	sort.Sort(sort.Reverse(sort.StringSlice(subvolsReversed)))

	for _, subvol := range subvols {
		// Since we create snapshots as readonly above, we must re-mark them
		// writable here before we can delete them.
		output, err := exec.Command(
			"btrfs",
			"property",
			"set",
			"-ts",
			path.Join(root, subvol),
			"ro",
			"false").CombinedOutput()
		if err != nil {
			return errors.Errorf("btrfs mark writable: %s: %s", err, output)
		}
	}

	for _, subvol := range subvolsReversed {
		output, err := exec.Command(
			"btrfs",
			"subvolume",
			"delete",
			"-c",
			path.Join(root, subvol)).CombinedOutput()
		if err != nil {
			return errors.Errorf("btrfs delete: %s: %s", err, output)
		}

		err = os.RemoveAll(path.Join(root, subvol))
		if err != nil {
			return errors.Wrapf(err, "failed to delete subvolume %s", subvol)
		}
	}

	return nil
}

func (b *btrfs) Delete(source string) error {
	return btrfsSubVolumesDelete(path.Join(b.c.RootFSDir, source))
}

func (b *btrfs) Detach() error {
	if b.needsUmount {
		// Need to use DETACH here because we still hold the rootfs .lock file.
		err := syscall.Unmount(b.c.RootFSDir, syscall.MNT_DETACH)
		err2 := os.RemoveAll(b.c.RootFSDir)
		if err != nil {
			return err
		}

		if err2 != nil {
			return err2
		}
	}

	return nil
}

func (b *btrfs) Exists(thing string) bool {
	_, err := os.Stat(path.Join(b.c.RootFSDir, thing))
	return err == nil
}

func (b *btrfs) TemporaryWritableSnapshot(source string) (string, func(), error) {
	dir, err := ioutil.TempDir(b.c.RootFSDir, fmt.Sprintf("temp-snapshot-%s-", source))
	if err != nil {
		return "", nil, errors.Wrapf(err, "couldn't create temporary snapshot dir for %s", source)
	}

	err = os.RemoveAll(dir)
	if err != nil {
		return "", nil, errors.Wrapf(err, "couldn't remove tempdir for %s", source)
	}

	dir = path.Base(dir)

	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"snapshot",
		path.Join(b.c.RootFSDir, source),
		path.Join(b.c.RootFSDir, dir)).CombinedOutput()
	if err != nil {
		return "", nil, errors.Errorf("temporary snapshot %s to %s: %s: %s", source, dir, err, string(output))
	}

	cleanup := func() {
		err = b.Delete(dir)
		if err != nil {
			log.Infof("problem deleting temp subvolume %s: %v", dir, err)
			return
		}
		err = os.RemoveAll(dir)
		if err != nil {
			log.Infof("problem deleting temp subvolume dir %s: %v", dir, err)
		}
	}

	return dir, cleanup, nil
}

func (b *btrfs) Clean() error {
	subvolErr := btrfsSubVolumesDelete(b.c.RootFSDir)
	loopback := path.Join(b.c.StackerDir, "btrfs.loop")

	var umountErr error
	_, err := os.Stat(loopback)
	if err == nil {
		// if we are inside a userns we can't unmount the loopback
		// (probably because someone did `sudo stacker unpriv-setup`);
		// they'll need to be root to unmount it as well.
		if shared.RunningInUserNS() {
			return errors.Errorf("can't fully clean btrfs from userns (try stacker clean ... as root)")
		}

		// Need to use DETACH here because we still hold the rootfs .lock file.
		umountErr = errors.Wrapf(syscall.Unmount(b.c.RootFSDir, syscall.MNT_DETACH), "unable to umount rootfs")
		if err = os.RemoveAll(loopback); err != nil {
			log.Infof("failed removing btrfs loopback file: %v", err)
		}
	}
	if err = os.RemoveAll(b.c.RootFSDir); err != nil {
		log.Infof("failed removing roots dir: %v", err)
	}
	if subvolErr != nil && umountErr != nil {
		return errors.Errorf("both subvol delete and umount failed: %v, %v", subvolErr, umountErr)
	}

	if subvolErr != nil {
		return subvolErr
	}

	return umountErr
}

func (b *btrfs) GetLXCRootfsConfig(name string) (string, error) {
	return fmt.Sprintf("dir:%s", path.Join(b.c.RootFSDir, name, "rootfs")), nil
}

func (b *btrfs) TarExtractLocation(name string) string {
	return path.Join(b.c.RootFSDir, name, "rootfs")
}

// MakeLoopbackBtrfs creates a btrfs filesystem mounted at dest out of a loop
// device and allows the specified uid to delete subvolumes on it.
func MakeLoopbackBtrfs(loopback string, size int64, uid int, gid int, dest string) error {
	mounted, err := mount.IsMountpoint(dest)
	if err != nil {
		return err
	}

	/* if it's already mounted, don't do anything */
	if mounted {
		return nil
	}

	if err := setupLoopback(loopback, uid, gid, size); err != nil {
		return err
	}

	/* Now we know that file is a valid btrfs "file" and that it's
	 * not mounted, so let's mount it.
	 */
	dev, err := attachToLoop(loopback)
	if err != nil {
		return errors.Errorf("Failed to attach loop device: %v", err)
	}
	defer dev.Detach()

	err = syscall.Mount(dev.Path(), dest, "btrfs", 0, "user_subvol_rm_allowed")
	if err != nil {
		return errors.Errorf("Failed mount fs: %v", err)
	}

	if err := os.Chown(dest, uid, gid); err != nil {
		return errors.Errorf("couldn't chown %s: %v", dest, err)
	}

	return nil
}

// attachToLoop attaches the path to a loop device, retrying for a while if it
// gets -EBUSY.
func attachToLoop(path string) (dev losetup.Device, err error) {
	// We can race between when we ask the kernel which loop device
	// is free and when we actually attach to it. This window is
	// pretty small, but still happens e.g. when we run the stacker
	// test suite. So let's sleep for a random number of ms and
	// retry the whole process again.
	for i := 0; i < 10; i++ {
		dev, err = losetup.Attach(path, 0, false)
		if err == nil {
			return dev, nil
		}

		// time.Durations are nanoseconds
		ms := rand.Int63n(100 * 1000 * 1000)
		time.Sleep(time.Duration(ms))
	}

	return dev, errors.Wrapf(err, "couldn't attach btrfs loop, too many retries")
}

func setupLoopback(path string, uid int, gid int, size int64) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}

		return nil
	}
	defer f.Close()

	if err := f.Chown(uid, gid); err != nil {
		os.RemoveAll(f.Name())
		return err
	}

	/* TODO: make this configurable */
	if err := syscall.Ftruncate(int(f.Fd()), size); err != nil {
		os.RemoveAll(f.Name())
		return err
	}

	output, err := exec.Command("mkfs.btrfs", f.Name()).CombinedOutput()
	if err != nil {
		os.RemoveAll(f.Name())
		return errors.Errorf("mkfs.btrfs: %s: %s", err, output)
	}

	return nil
}

func (b *btrfs) SetOverlayDirs(name string, overlayDirs types.OverlayDirs, layerTypes []types.LayerType) error {
	if len(overlayDirs) == 0 {
		return nil
	}
	return errors.Errorf("Using overlay_dirs with btrfs storage is forbidden, use overlay storage instead")
}
