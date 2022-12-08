package stacker

import (
	"bytes"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/types"
)

func findLock(st *syscall.Stat_t) error {
	content, err := os.ReadFile("/proc/locks")
	if err != nil {
		return errors.Wrapf(err, "failed to read locks file")
	}

	for _, line := range strings.Split(string(content), "\n") {
		if len(line) == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			return errors.Errorf("invalid lock file entry %s", line)
		}

		entries := strings.Split(fields[5], ":")
		if len(entries) != 3 {
			return errors.Errorf("invalid lock file field %s", fields[5])
		}

		/*
		 * XXX: the kernel prints "fd:01:$ino" for some (all?) locks,
		 * even though the man page we should be able to use fields 0
		 * and 1 as major and minor device types. Let's just ignore
		 * these.
		 */

		ino, err := strconv.ParseUint(entries[2], 10, 64)
		if err != nil {
			return errors.Wrapf(err, "invalid ino %s", entries[2])
		}

		if st.Ino != ino {
			continue
		}

		pid := fields[4]
		content, err := os.ReadFile(path.Join("/proc", pid, "cmdline"))
		if err != nil {
			return errors.Errorf("lock owned by pid %s", pid)
		}

		content = bytes.Replace(content, []byte{0}, []byte{' '}, -1)
		return errors.Errorf("lock owned by pid %s (%s)", pid, string(content))
	}

	return errors.Errorf("couldn't find who owns the lock")
}

func acquireLock(p string) (*os.File, error) {
	lockfile, err := os.Create(p)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't create lockfile %s", p)
	}

	lockMode := syscall.LOCK_EX

	lockErr := syscall.Flock(int(lockfile.Fd()), lockMode|syscall.LOCK_NB)
	if lockErr == nil {
		return lockfile, nil
	}

	fi, err := lockfile.Stat()
	lockfile.Close()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't lock or stat lockfile %s", p)
	}

	owner := findLock(fi.Sys().(*syscall.Stat_t))
	return nil, errors.Wrapf(lockErr, "couldn't acquire lock on %s: %v", p, owner)
}

const lockPath = ".lock"

type StackerLocks struct {
	stackerDir, rootsDir *os.File
}

func (ls *StackerLocks) Unlock() {
	// TODO: it would be good to lock the OCI dir here, because I can
	// imagine two people trying to output stuff to the same directory.
	// However, that screws with umoci, because it sees an empty dir as an
	// invalid image. the bug we're trying to fix <hair on fire>right
	// now</hair on fire> is multiple invocations on a roots dir, so this
	// is good enough.
	for _, lock := range []*os.File{ls.stackerDir, ls.rootsDir} {
		if lock != nil {
			lock.Close()
		}
	}
	ls.stackerDir = nil
	ls.rootsDir = nil
}

func lock(config types.StackerConfig) (*StackerLocks, error) {
	ls := &StackerLocks{}

	var err error
	ls.stackerDir, err = acquireLock(path.Join(config.StackerDir, lockPath))
	if err != nil {
		return nil, err
	}

	ls.rootsDir, err = acquireLock(path.Join(config.RootFSDir, lockPath))
	if err != nil {
		ls.Unlock()
		return nil, err
	}

	return ls, nil
}
