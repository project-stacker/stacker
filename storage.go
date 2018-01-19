package stacker

import (
	"bufio"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/freddierice/go-losetup"
)

type DiffStrategy int

const (
	NativeDiff DiffStrategy = iota
	TarDiff    DiffStrategy = iota
)

type Storage interface {
	Name() string
	Create(path string) error
	Snapshot(source string, target string) error
	Restore(source string, target string) error
	Delete(path string) error

	// Diff returns a reader for the blob to put, and a hash for the
	// uncompressed layer if it is uncompressed. Note that OCI uses the
	// uncompressed hash for the diffID field, and the compressed hash as
	// the actual blob value, so unfortunately we need both.
	Diff(DiffStrategy, string, string) (io.ReadCloser, hash.Hash, error)
	Undiff(DiffStrategy, string, io.Reader) error
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

		stackermount := "stackermount"
		if _, err := exec.LookPath(stackermount); err != nil {
			link, err := os.Readlink("/proc/self/exe")
			if err != nil {
				return nil, err
			}

			stackermount = path.Join(path.Dir(link), "stackermount")
		}

		// If it's not btrfs, let's make it one via a loopback.
		// TODO: make the size configurable
		output, err := exec.Command(
			stackermount,
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
	fmt.Printf("restoring %s to %s\n", source, target)
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

func (b *btrfs) Delete(source string) error {
	output, err := exec.Command(
		"btrfs",
		"subvolume",
		"delete",
		path.Join(b.c.RootFSDir, source)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs delete: %s: %s", err, output)
	}

	return os.RemoveAll(path.Join(b.c.RootFSDir, source))
}

type cmdRead struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
	done   bool
}

func (crc *cmdRead) Read(p []byte) (int, error) {
	if crc.done {
		return 0, io.EOF
	}

	n, err := crc.stdout.Read(p)
	if err == io.EOF {
		crc.done = true
		content, err2 := ioutil.ReadAll(crc.stderr)
		err := crc.cmd.Wait()
		if err != nil {
			if err2 == nil {
				return n, fmt.Errorf("EOF and %s: %s", err, string(content))
			}

			return n, fmt.Errorf("EOF and %s", err)
		}
	}

	return n, err
}

func (crc *cmdRead) Close() error {
	crc.stdout.Close()
	crc.stderr.Close()
	if !crc.done {
		return crc.cmd.Wait()
	}

	return nil
}

func (b *btrfs) nativeDiff(source string, target string) (io.ReadCloser, hash.Hash, error) {
	// for now we can ignore strategy, since there is only one
	args := []string{"send"}
	if source != "" {
		args = append(args, "-p", path.Join(b.c.RootFSDir, source))
	}
	args = append(args, path.Join(b.c.RootFSDir, target))

	cmd := exec.Command("btrfs", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	return &cmdRead{cmd: cmd, stdout: stdout, stderr: stderr}, nil, nil
}

func (b *btrfs) Diff(strategy DiffStrategy, source string, target string) (io.ReadCloser, hash.Hash, error) {
	switch strategy {
	case NativeDiff:
		return b.nativeDiff(source, target)
	case TarDiff:
		return tarDiff(b.c, source, target)
	default:
		return nil, nil, fmt.Errorf("unknown diff strategy")
	}
}

func (b *btrfs) nativeUndiff(name string, r io.Reader) error {
	cmd := exec.Command("btrfs", "receive", "-e", b.c.RootFSDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer stderr.Close()

	err = cmd.Start()
	if err != nil {
		return err
	}

	_, err = io.Copy(stdin, r)
	if err != nil {
		return err
	}

	content, err2 := ioutil.ReadAll(stderr)
	err = cmd.Wait()
	if err != nil {
		if err2 == nil {
			return fmt.Errorf("btrfs receive: %s: %s", err, string(content))
		}

		return fmt.Errorf("btrfs receive: %s", err)
	}

	return nil
}

func (b *btrfs) Undiff(strategy DiffStrategy, name string, r io.Reader) error {
	switch strategy {
	case NativeDiff:
		return b.nativeUndiff(name, r)
	case TarDiff:
		return fmt.Errorf("TarDiff unpack not implemented")
	default:
		return fmt.Errorf("unknown undiff strategy")
	}
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

func MediaTypeToDiffStrategy(mt string) (DiffStrategy, error) {
	switch mt {
	case MediaTypeImageBtrfsLayer:
		return NativeDiff, nil
	case ispec.MediaTypeImageLayerGzip:
		return TarDiff, nil
	default:
		return 0, fmt.Errorf("unknown media type: %s", mt)
	}
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
