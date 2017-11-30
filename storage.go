package stacker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
	"strings"
)

type Storage interface {
	Name() string
	Init() error
	Snapshot(hash string) error
	Restore(hash string) error
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
	if fs.Type != 0x9123683E {
		return &btrfsLoopback{c: c}, nil
	}

	return nil, fmt.Errorf("not implemented")
}

type btrfsLoopback struct {
	c StackerConfig

	loopback string
}

func (b *btrfsLoopback) Name() string {
	return "btrfs loopback"
}

func (b *btrfsLoopback) Init() error {
	if err := os.MkdirAll(b.c.StackerDir, 0755); err != nil {
		return err
	}

	b.loopback = path.Join(b.c.StackerDir, "btrfs.loop")

	f, err := os.OpenFile(b.loopback, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}

		/* It existed, was it mounted too? */
		f, err := os.Open("/proc/self/mountinfo")
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, b.loopback) {
				return nil
			}
		}
	} else {
		/* TODO: make this configurable */
		err := syscall.Ftruncate(int(f.Fd()), 100 * 1024 * 1024 * 1024)
		f.Close()
		if err != nil {
			return err
		}

		output, err := exec.Command("mkfs.btrfs", f.Name()).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mkfs.btrfs: %s: %s", err, output)
		}
	}

	/* Now we know that b.loopback is a valid btrfs "file" and that it's
	 * not mounted, so let's mount it.
	 * FIXME: this should probably be done in golang, but it's more work to
	 * set up the loopback mounts.
	 */
	output, err := exec.Command("mount", "-o", "loop", b.loopback, b.c.RootFSDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("problem doing loopback mount: %s: %s", err, output)
	}
	return nil
}

func (b *btrfsLoopback) Snapshot(hash string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"snapshot",
		"-r",
		b.c.RootFSDir,
		hash).CombinedOutput()

	if err != nil {
		return fmt.Errorf("btrfs snapshot: %s: %s", err, output)
	}

	return nil
}

func (b *btrfsLoopback) Restore(hash string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"snapshot",
		hash,
		b.c.RootFSDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs snapshot: %s: %s", err, output)
	}

	return nil
}

func (b *btrfsLoopback) Detach() error {
	return syscall.Unmount(b.c.RootFSDir, 0)
}
