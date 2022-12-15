package atomfs

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"stackerbuild.io/stacker/pkg/mount"
	"stackerbuild.io/stacker/pkg/squashfs"
)

type Molecule struct {
	// Atoms is the list of atoms in this Molecule. The first element in
	// this list is the top most layer in the overlayfs.
	Atoms []ispec.Descriptor

	config MountOCIOpts
}

// mountUnderlyingAtoms mounts all the underlying atoms at
// config.MountedAtomsPath().
func (m Molecule) mountUnderlyingAtoms() error {
	for _, a := range m.Atoms {
		target := m.config.MountedAtomsPath(a.Digest.Encoded())

		rootHash := a.Annotations[squashfs.VerityRootHashAnnotation]

		if !m.config.AllowMissingVerityData && rootHash == "" {
			return errors.Errorf("%v is missing verity data", a.Digest)
		}

		mounts, err := mount.ParseMounts("/proc/self/mountinfo")
		if err != nil {
			return err
		}

		mountpoint, mounted := mounts.FindMount(target)

		if mounted {
			err = squashfs.ConfirmExistingVerityDeviceHash(mountpoint.Source, rootHash)
			if err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(target, 0755); err != nil {
			return err
		}

		err = squashfs.Mount(m.config.AtomsPath(a.Digest.Encoded()), target, rootHash)
		if err != nil {
			return errors.Wrapf(err, "couldn't mount")
		}
	}

	return nil
}

// overlayArgs returns all of the mount options to pass to the kernel to
// actually mount this molecule.
func (m Molecule) overlayArgs(dest string) (string, error) {
	dirs := []string{}
	for _, a := range m.Atoms {
		target := m.config.MountedAtomsPath(a.Digest.Encoded())
		dirs = append(dirs, target)
	}

	// overlay doesn't work with one lowerdir. so we do a hack here: we
	// just create an empty directory called "workaround" in the mounts
	// directory, and add that to the dir list if it's of length one.
	if len(dirs) == 1 {
		workaround := m.config.MountedAtomsPath("workaround")
		if err := os.MkdirAll(workaround, 0755); err != nil {
			return "", errors.Wrapf(err, "couldn't make workaround dir")
		}

		dirs = append(dirs, workaround)
	}

	// Note that in overlayfs, the first thing is the top most layer in the
	// overlay.
	mntOpts := "index=off,lowerdir=" + strings.Join(dirs, ":")
	return mntOpts, nil
}

// device mapper has no namespacing. if two different binaries invoke this code
// (for example, the stacker test suite), we want to be sure we don't both
// create or delete devices out from the other one when they have detected the
// device exists. so try to cooperate via this lock.
var advisoryLockPath = path.Join(os.TempDir(), ".atomfs-lock")

func (m Molecule) Mount(dest string) error {
	lockfile, err := os.Create(advisoryLockPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer lockfile.Close()

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX)
	if err != nil {
		return errors.WithStack(err)
	}

	mntOpts, err := m.overlayArgs(dest)
	if err != nil {
		return err
	}

	// The kernel doesn't allow mount options longer than 4096 chars, so
	// let's give a nicer error than -EINVAL here.
	if len(mntOpts) > 4096 {
		return errors.Errorf("too many lower dirs; must have fewer than 4096 chars")
	}

	err = m.mountUnderlyingAtoms()
	if err != nil {
		return err
	}

	// now, do the actual overlay mount
	err = unix.Mount("overlay", dest, "overlay", 0, mntOpts)
	return errors.Wrapf(err, "couldn't do overlay mount to %s, opts: %s", dest, mntOpts)
}

func Umount(dest string) error {
	var err error
	dest, err = filepath.Abs(dest)
	if err != nil {
		return errors.Wrapf(err, "couldn't create abs path for %v", dest)
	}

	lockfile, err := os.Create(advisoryLockPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer lockfile.Close()

	err = unix.Flock(int(lockfile.Fd()), unix.LOCK_EX)
	if err != nil {
		return errors.WithStack(err)
	}

	mounts, err := mount.ParseMounts("/proc/self/mountinfo")
	if err != nil {
		return err
	}

	underlyingAtoms := []string{}
	for _, m := range mounts {
		if m.FSType != "overlay" {
			continue
		}

		if m.Target != dest {
			continue
		}

		underlyingAtoms, err = m.GetOverlayDirs()
		if err != nil {
			return err
		}
		break
	}

	if len(underlyingAtoms) == 0 {
		return errors.Errorf("%s is not an atomfs mountpoint", dest)
	}

	if err := unix.Unmount(dest, 0); err != nil {
		return err
	}

	// now, "refcount" the remaining atoms and see if any of ours are
	// unused
	usedAtoms := map[string]int{}

	mounts, err = mount.ParseMounts("/proc/self/mountinfo")
	if err != nil {
		return err
	}

	for _, m := range mounts {
		if m.FSType != "overlay" {
			continue
		}

		dirs, err := m.GetOverlayDirs()
		if err != nil {
			return err
		}
		for _, d := range dirs {
			usedAtoms[d]++
		}
	}

	// If any of the atoms underlying the target mountpoint are now unused,
	// let's unmount them too.
	for _, a := range underlyingAtoms {
		_, used := usedAtoms[a]
		if used {
			continue
		}
		/* TODO: some kind of logging
		if !used {
			log.Warnf("unused atom %s was part of this molecule?")
			continue
		}
		*/

		// the workaround dir isn't really a mountpoint, so don't unmount it
		if path.Base(a) == "workaround" {
			continue
		}

		err = squashfs.Umount(a)
		if err != nil {
			return err
		}
	}

	return nil
}
