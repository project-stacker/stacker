package lib

import (
	"io"
	"os"
)

func FileCopy(dest string, source string) error {
	os.RemoveAll(dest)

	linkFI, err := os.Lstat(source)
	if err != nil {
		return err
	}

	// If it's a link, it might be broken. In any case, just copy it.
	if linkFI.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}

		return os.Symlink(target, dest)
	}

	s, err := os.Open(source)
	if err != nil {
		return err
	}
	defer s.Close()

	fi, err := s.Stat()
	if err != nil {
		return err
	}

	d, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer d.Close()

	err = d.Chmod(fi.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(d, s)
	return err
}
