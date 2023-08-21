package lib

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
)

func FileCopy(dest string, source string, mode *fs.FileMode, uid, gid int) error {
	os.RemoveAll(dest)

	linkFI, err := os.Lstat(source)
	if err != nil {
		return errors.Wrapf(err, "Coudn't stat link %s", source)
	}

	// If it's a link, it might be broken. In any case, just copy it.
	if linkFI.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return errors.Wrapf(err, "Coudn't read link %s", source)
		}

		if err = os.Symlink(target, dest); err != nil {
			return errors.Wrapf(err, "Couldn't symlink %s->%s", source, target)
		}

		if err = os.Lchown(dest, uid, gid); err != nil {
			return errors.Wrapf(err, "Couldn't set symlink ownership %s", dest)
		}

		return nil
	}

	s, err := os.Open(source)
	if err != nil {
		return errors.Wrapf(err, "Coudn't open file %s", source)
	}
	defer s.Close()

	fi, err := s.Stat()
	if err != nil {
		return errors.Wrapf(err, "Coudn't stat file %s", source)
	}

	d, err := os.Create(dest)
	if err != nil {
		return errors.Wrapf(err, "Coudn't create file %s", dest)
	}
	defer d.Close()

	if mode != nil {
		err = d.Chmod(*mode)
	} else {
		err = d.Chmod(fi.Mode())
	}
	if err != nil {
		return errors.Wrapf(err, "Coudn't chmod file %s", source)
	}

	err = d.Chown(uid, gid)
	if err != nil {
		return errors.Wrapf(err, "Coudn't chown file %s", source)
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

		if err != nil {
			return err
		}

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
