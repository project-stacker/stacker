package stacker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/freddierice/go-losetup"
)

type Storage interface {
	Name() string
	Create(path string) error
	Snapshot(source string, target string) error
	Restore(source string, target string) error
	Delete(path string) error
	Detach() error
}

func NewStorage(c StackerConfig) (Storage, error) {
	fs := syscall.Statfs_t{}

	if err := os.MkdirAll(c.RootFSDir, 0755); err != nil {
		return nil, err
	}

	err := syscall.Statfs(c.RootFSDir, &fs)
	if err != nil {
		return nil, err
	}

	/* btrfs superblock magic number */
	isBtrfs := fs.Type == 0x9123683E

	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}

	if !isBtrfs {
		if err := os.MkdirAll(c.StackerDir, 0755); err != nil {
			return nil, err
		}

		loopback := path.Join(c.StackerDir, "btrfs.loop")
		size := 100 * 1024 * 1024 * 1024
		uid, err := strconv.Atoi(currentUser.Uid)
		if err != nil {
			return nil, err
		}

		err = MakeLoopbackBtrfs(loopback, int64(size), uid, c.RootFSDir)
		if err != nil {
			return nil, err
		}

	}

	return &btrfs{c: c, needsUmount: !isBtrfs}, nil
}

type btrfs struct {
	c           StackerConfig
	needsUmount bool
}

func (b *btrfs) Name() string {
	return "btrfs"
}

func (b *btrfs) Create(source string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"create",
		path.Join(b.c.RootFSDir, source)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs create: %s: %s", err, output)
	}

	return nil
}

func (b *btrfs) Snapshot(source string, target string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"snapshot",
		"-r",
		path.Join(b.c.RootFSDir, source),
		path.Join(b.c.RootFSDir, target)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs snapshot: %s: %s", err, output)
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
		return fmt.Errorf("btrfs restore: %s: %s", err, output)
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
		return fmt.Errorf("btrfs mark writable: %s: %s", err, output)
	}

	return nil
}

// These next three functions are lifted from LXD, which is also under the
// apache2 license.

// isBtrfsSubVolume returns true if the given Path is a btrfs subvolume else
// false.
func isBtrfsSubVolume(subvolPath string) bool {
	fs := syscall.Stat_t{}
	err := syscall.Lstat(subvolPath, &fs)
	if err != nil {
		return false
	}

	// Check if BTRFS_FIRST_FREE_OBJECTID
	if fs.Ino != 256 {
		return false
	}

	return true
}

func btrfsSubVolumesGet(path string) ([]string, error) {
	result := []string{}

	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// Unprivileged users can't get to fs internals
	filepath.Walk(path, func(fpath string, fi os.FileInfo, err error) error {
		// Skip walk errors
		if err != nil {
			return nil
		}

		// Ignore the base path
		if strings.TrimRight(fpath, "/") == strings.TrimRight(path, "/") {
			return nil
		}

		// Subvolumes can only be directories
		if !fi.IsDir() {
			return nil
		}

		// Check if a btrfs subvolume
		if isBtrfsSubVolume(fpath) {
			result = append(result, strings.TrimPrefix(fpath, path))
		}

		return nil
	})

	return result, nil
}

// btrfsPoolVolumesDelete is the recursive variant on btrfsPoolVolumeDelete,
// it first deletes subvolumes of the subvolume and then the
// subvolume itself.
func btrfsSubVolumesDelete(subvol string) error {
	// Delete subsubvols.
	subsubvols, err := btrfsSubVolumesGet(subvol)
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(subsubvols)))

	for _, subsubvol := range subsubvols {
		err := btrfsSubVolumeDelete(path.Join(subvol, subsubvol))
		if err != nil {
			return err
		}
	}

	// Delete the subvol itself
	err = btrfsSubVolumeDelete(subvol)
	if err != nil {
		return err
	}

	return nil
}

func btrfsSubVolumeDelete(subvol string) error {
	// Since we create snapshots as readonly above, we must re-mark them
	// writable here before we can delete them.
	output, err := exec.Command(
		"btrfs",
		"property",
		"set",
		"-ts",
		subvol,
		"ro",
		"false").CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs mark writable: %s: %s", err, output)
	}

	output, err = exec.Command(
		"btrfs",
		"subvolume",
		"delete",
		subvol).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs delete: %s: %s", err, output)
	}

	return os.RemoveAll(subvol)
}

func (b *btrfs) Delete(source string) error {
	return btrfsSubVolumesDelete(path.Join(b.c.RootFSDir, source))
}

func (b *btrfs) Detach() error {
	if b.needsUmount {
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

// MakeLoopbackBtrfs creates a btrfs filesystem mounted at dest out of a loop
// device and allows the specified uid to delete subvolumes on it.
func MakeLoopbackBtrfs(loopback string, size int64, uid int, dest string) error {
	mounted, err := isMounted(loopback)
	if err != nil {
		return err
	}

	/* if it's already mounted, don't do anything */
	if mounted {
		return nil
	}

	if err := setupLoopback(loopback, uid, size); err != nil {
		return err
	}

	/* Now we know that file is a valid btrfs "file" and that it's
	 * not mounted, so let's mount it.
	 */
	dev, err := losetup.Attach(loopback, 0, false)
	if err != nil {
		return fmt.Errorf("Failed to attach loop device: %v", err)
	}
	defer dev.Detach()

	err = syscall.Mount(dev.Path(), dest, "btrfs", 0, "user_subvol_rm_allowed,flushoncommit")
	if err != nil {
		return fmt.Errorf("Failed mount fs: %v", err)
	}

	if err := os.Chown(dest, uid, uid); err != nil {
		return fmt.Errorf("couldn't chown %s: %v", dest, err)
	}

	return nil
}

func setupLoopback(path string, uid int, size int64) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}

		return nil
	}
	defer f.Close()

	if err := f.Chown(uid, uid); err != nil {
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
		return fmt.Errorf("mkfs.btrfs: %s: %s", err, output)
	}

	return nil
}

func isMounted(path string) (bool, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, path) {
			return true, nil
		}
	}

	return false, nil
}

func haveRsync() error {
	_, err := exec.LookPath("rsync")
	if err != nil {
		return fmt.Errorf("rsync binary not found")
	}

	return nil
}
