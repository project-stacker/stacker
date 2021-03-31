package lib

import (
	"io/ioutil"
	"os"
	"path"
)

// DirCopy copies a whole directory recursively
func DirCopy(dest string, source string) error {

	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(source); err != nil {
		return err
	}

	linkFI, err := os.Lstat(source)
	if err != nil {
		return err
	}

	// in case the dir is a symlink
	if linkFI.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}

		return os.Symlink(target, dest)
	}

	//var dirMode os.FileMode
	//if preservePermissions {
	//	dirMode = srcinfo.Mode()
	//} else {
	//	dirMode = 0755
	//}

	dirMode := srcinfo.Mode()

	if err = os.MkdirAll(dest, dirMode); err != nil {
		return err
	}

	if fds, err = ioutil.ReadDir(source); err != nil {
		return err
	}

	for _, fd := range fds {
		srcfp := path.Join(source, fd.Name())
		dstfp := path.Join(dest, fd.Name())

		if fd.IsDir() {
			if err = DirCopy(dstfp, srcfp); err != nil {
				return err
			}
		} else {
			if err = FileCopy(dstfp, srcfp); err != nil {
				return err
			}
		}
	}
	return nil
}
