package stacker

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"syscall"
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

		// If it's not btrfs, let's make it one via a loopback.
		// TODO: make the size configurable
		output, err := exec.Command(
			"stackermount",
			path.Join(c.StackerDir, "btrfs.loop"),
			fmt.Sprintf("%d", 100*1024*1024*1024),
			currentUser.Uid,
			c.RootFSDir,
		).CombinedOutput()
		if err != nil {
			os.RemoveAll(c.StackerDir)
			return nil, fmt.Errorf("creating loopback: %s: %s", err, output)
		}
	} else {
		// If it *is* btrfs, let's make sure we can actually create
		// subvolumes like we need to.
		fi, err := os.Stat(c.RootFSDir)
		if err != nil {
			return nil, err
		}

		myUid, err := strconv.Atoi(currentUser.Uid)
		if err != nil {
			return nil, err
		}

		if fi.Sys().(*syscall.Stat_t).Uid != uint32(myUid) {
			return nil, fmt.Errorf(
				"%s must be owned by you. try `sudo chmod %s %s`",
				c.RootFSDir,
				currentUser.Uid,
				c.RootFSDir)
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

	return nil
}

func (b *btrfs) Delete(source string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"delete",
		path.Join(b.c.RootFSDir, source)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs delete: %s: %s", err, output)
	}

	return nil
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
