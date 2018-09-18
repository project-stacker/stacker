package stacker

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"

	"github.com/pkg/errors"
	"github.com/udhos/equalfile"
	"github.com/vbatts/go-mtree"
)

func fileCopy(dest string, source string) error {
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

// filesDiffer returns true if the files are different, false if they are the same.
func filesDiffer(p1 string, info1 os.FileInfo, p2 string, info2 os.FileInfo) (bool, error) {
	if info1.Name() != info2.Name() {
		return false, fmt.Errorf("comparing files without the same name?")
	}

	if info1.Mode()&os.ModeSymlink != 0 {
		if info2.Mode()&os.ModeSymlink != 0 {
			link1, err := os.Readlink(p1)
			if err != nil {
				return false, err
			}

			link2, err := os.Readlink(p2)
			if err != nil {
				return false, err
			}
			return link1 != link2, err
		}

		return false, fmt.Errorf("symlink -> not symlink not supported")
	}

	if info1.Size() != info2.Size() {
		return true, nil
	}

	f1, err := os.Open(p1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(p2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	eq, err := equalfile.New(nil, equalfile.Options{}).CompareReader(f1, f2)
	if err != nil {
		return false, err
	}

	return !eq, nil
}

func importFile(imp string, cacheDir string) (string, error) {
	e1, err := os.Stat(imp)
	if err != nil {
		return "", err
	}

	if !e1.IsDir() {
		needsCopy := false
		dest := path.Join(cacheDir, path.Base(imp))
		e2, err := os.Stat(dest)
		if err != nil {
			needsCopy = true
		} else {
			differ, err := filesDiffer(imp, e1, dest, e2)
			if err != nil {
				return "", err
			}

			needsCopy = differ
		}

		if needsCopy {
			fmt.Printf("copying %s\n", imp)
			if err := fileCopy(dest, imp); err != nil {
				return "", errors.Wrapf(err, "couldn't copy import %s", imp)
			}
		} else {
			fmt.Println("using cached copy of", imp)
		}

		return dest, nil
	}

	existing, err := walkImport(cacheDir)
	if err != nil {
		return "", errors.Wrapf(err, "failed walking existing import dir")
	}

	dest := path.Join(cacheDir, path.Base(imp))
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}

	toImport, err := walkImport(imp)
	if err != nil {
		return "", errors.Wrapf(err, "failed walking dir to import")
	}

	diff, err := mtree.Compare(existing, toImport, mtreeKeywords)
	if err != nil {
		return "", err
	}

	for _, d := range diff {
		switch d.Type() {
		case mtree.Missing:
			err := os.RemoveAll(path.Join(dest, d.Path()))
			if err != nil {
				return "", err
			}
		case mtree.Modified:
			fallthrough
		case mtree.Extra:
			err := os.RemoveAll(path.Join(dest, d.Path()))
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}

			output, err := exec.Command("cp", "-a", path.Join(imp, d.Path()), path.Join(dest, d.Path())).CombinedOutput()
			if err != nil {
				return "", errors.Wrapf(err, "couldn't copy %s: %s", path.Join(imp, d.Path()), string(output))
			}
		case mtree.ErrorDifference:
			return "", errors.Errorf("failed to diff %s", d.Path())
		}
	}

	return dest, nil

}

func acquireUrl(c StackerConfig, i string, cache string) (string, error) {
	url, err := url.Parse(i)
	if err != nil {
		return "", err
	}

	// It's just a path, let's copy it to .stacker.
	if url.Scheme == "" {
		return importFile(i, cache)
	} else if url.Scheme == "http" || url.Scheme == "https" {
		// otherwise, we need to download it
		return download(cache, i)
	} else if url.Scheme == "stacker" {
		p := path.Join(c.RootFSDir, url.Host, "rootfs", url.Path)
		return importFile(p, cache)
	}

	return "", fmt.Errorf("unsupported url scheme %s", i)
}

func Import(c StackerConfig, name string, imports []string) error {
	dir := path.Join(c.StackerDir, "imports", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	existing, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, i := range imports {
		name, err := acquireUrl(c, i, dir)
		if err != nil {
			return err
		}

		for i, ext := range existing {
			if ext.Name() == path.Base(name) {
				existing = append(existing[:i], existing[i+1:]...)
				break
			}
		}
	}

	// Now, delete all the old imports.
	for _, ext := range existing {
		err = os.RemoveAll(path.Join(dir, ext.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}
