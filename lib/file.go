package lib

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
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

// FindFiles searches for paths matching a particular regex under a given folder
func FindFiles(base, pattern string) ([]string, error) {
	var err error
	var paths []string

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	visit := func(path string, info os.FileInfo, err error) error {

		if info.IsDir() {
			return nil
		}

		matched := re.MatchString(path)

		if matched {
			paths = append(paths, path)
		}

		return nil
	}

	// Note symlinks are not followed by walk implementation
	err = filepath.Walk(base, visit)

	return paths, err
}
