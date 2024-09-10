package lib

import (
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// DirCopy copies a whole directory recursively
func DirCopy(dest string, source string) error {

	var err error
	var fds []os.DirEntry
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(source); err != nil {
		return errors.Wrapf(err, "Coudn't stat %s", source)
	}

	linkFI, err := os.Lstat(source)
	if err != nil {
		return errors.Wrapf(err, "Coudn't stat link %s", source)
	}

	// in case the dir is a symlink
	if linkFI.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return errors.Wrapf(err, "Coudn't read link %s", source)
		}

		return os.Symlink(target, dest)
	}

	dirMode := srcinfo.Mode()

	if err = os.MkdirAll(dest, dirMode); err != nil {
		return errors.Wrapf(err, "Coudn't mkdir %s", dest)
	}

	if fds, err = os.ReadDir(source); err != nil {
		return errors.Wrapf(err, "Coudn't read dir %s", source)
	}

	for _, fd := range fds {
		srcfp := path.Join(source, fd.Name())
		dstfp := path.Join(dest, fd.Name())

		if fd.IsDir() {
			if err = DirCopy(dstfp, srcfp); err != nil {
				return err
			}
		} else {
			if err = FileCopyNoPerms(dstfp, srcfp); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyThing copies either a dir or file to the target.
func CopyThing(srcpath, destpath string) error {
	srcInfo, err := os.Lstat(srcpath)
	if err != nil {
		return errors.WithStack(err)
	}

	if srcInfo.IsDir() {
		return DirCopy(destpath, srcpath)
	} else {
		return FileCopy(destpath, srcpath, nil, -1, -1)
	}
}

// Chmod changes file permissions
func Chmod(mode, destpath string) error {
	destInfo, err := os.Lstat(destpath)
	if err != nil {
		return errors.WithStack(err)
	}

	if destInfo.IsDir() {
		return errors.WithStack(os.ErrInvalid)
	}

	if destInfo.Mode()&os.ModeSymlink != 0 {
		return errors.WithStack(os.ErrInvalid)
	}

	// read as an octal value
	iperms, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return errors.WithStack(err)
	}

	return os.Chmod(destpath, fs.FileMode(iperms))
}

// Chown changes file ownership
func Chown(owner, destpath string) error {
	destInfo, err := os.Lstat(destpath)
	if err != nil {
		return errors.WithStack(err)
	}

	if destInfo.IsDir() {
		return errors.WithStack(os.ErrInvalid)
	}

	owns := strings.Split(owner, ":")
	if len(owns) > 2 {
		return errors.WithStack(os.ErrInvalid)
	}

	uid, err := strconv.ParseInt(owns[0], 10, 32)
	if err != nil {
		return errors.WithStack(err)
	}

	gid := int64(-1)
	if len(owns) > 1 {
		gid, err = strconv.ParseInt(owns[1], 10, 32)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return os.Lchown(destpath, int(uid), int(gid))
}
