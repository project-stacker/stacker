package stacker

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/anuvu/stacker/lib"
	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
	"github.com/udhos/equalfile"
	"github.com/vbatts/go-mtree"
)

// filesDiffer returns true if the files are different, false if they are the same.
func filesDiffer(p1 string, info1 os.FileInfo, p2 string, info2 os.FileInfo) (bool, error) {
	if info1.Name() != info2.Name() {
		return false, errors.Errorf("comparing files without the same name?")
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

		return false, errors.Errorf("symlink -> not symlink not supported")
	}

	if info1.Size() != info2.Size() {
		return true, nil
	}

	if info1.Mode() != info2.Mode() {
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
	e1, err := os.Lstat(imp)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't stat import %s", imp)
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
			log.Infof("copying %s", imp)
			if err := lib.FileCopy(dest, imp); err != nil {
				return "", errors.Wrapf(err, "couldn't copy import %s", imp)
			}
		} else {
			log.Infof("using cached copy of %s", imp)
		}

		return dest, nil
	}

	dest := path.Join(cacheDir, path.Base(imp))
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", errors.Wrapf(err, "failed making cache dir")
	}

	existing, err := walkImport(dest)
	if err != nil {
		return "", errors.Wrapf(err, "failed walking existing import dir")
	}

	toImport, err := walkImport(imp)
	if err != nil {
		return "", errors.Wrapf(err, "failed walking dir to import")
	}

	diff, err := mtree.Compare(existing, toImport, mtreeKeywords)
	if err != nil {
		return "", errors.Wrapf(err, "failed mtree comparing %s and %s", existing, toImport)
	}

	for _, d := range diff {
		switch d.Type() {
		case mtree.Missing:
			err := os.RemoveAll(path.Join(cacheDir, path.Base(imp), d.Path()))
			if err != nil {
				return "", errors.Wrapf(err, "couldn't remove missing import %s", path.Join(cacheDir, path.Base(imp), d.Path()))
			}
		case mtree.Modified:
			fallthrough
		case mtree.Extra:
			srcpath := path.Join(imp, d.Path())
			destpath := path.Join(cacheDir, path.Base(imp), d.Path())

			err = os.RemoveAll(destpath)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}

			sdirinfo, err := os.Lstat(path.Dir(srcpath))
			if err != nil {
				return "", err
			}

			destdir := path.Dir(destpath)

			derr := os.MkdirAll(destdir, sdirinfo.Mode())
			if derr != nil {
				return "", errors.Wrapf(err, "failed to create dir %s", destdir)
			}

			output, err := exec.Command("cp", "-a", srcpath, destdir).CombinedOutput()
			if err != nil {
				return "", errors.Wrapf(err, "couldn't copy %s: %s", path.Join(imp, d.Path()), string(output))
			}
		case mtree.ErrorDifference:
			return "", errors.Errorf("failed to diff %s", d.Path())
		}
	}

	return dest, nil

}

func acquireUrl(c types.StackerConfig, i string, cache string, progress bool) (string, error) {
	url, err := newDockerishUrl(i)
	if err != nil {
		return "", err
	}

	// It's just a path, let's copy it to .stacker.
	if url.Scheme == "" {
		return importFile(i, cache)
	} else if url.Scheme == "http" || url.Scheme == "https" {
		// otherwise, we need to download it
		return Download(cache, i, progress)
	} else if url.Scheme == "stacker" {
		p := path.Join(c.RootFSDir, url.Host, "rootfs", url.Path)
		return importFile(p, cache)
	}

	return "", errors.Errorf("unsupported url scheme %s", i)
}

func CleanImportsDir(c types.StackerConfig, name string, imports []string, cache *BuildCache) error {
	dir := path.Join(c.StackerDir, "imports", name)

	cacheEntry, cacheHit := cache.Cache[name]
	if !cacheHit {
		// no previous build means we should delete everything that was
		// imported; who knows where it came from.
		return os.RemoveAll(dir)
	}

	// If the base name of two things was the same across builds
	// but the URL they were imported from was different, let's
	// make sure we invalidate the cached version.
	for _, i := range imports {
		for cached := range cacheEntry.Imports {
			if path.Base(cached) == path.Base(i) && cached != i {
				log.Infof("%s url changed to %s, pruning cache", cached, i)
				err := os.RemoveAll(path.Join(dir, path.Base(i)))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func Import(c types.StackerConfig, name string, imports []string, progress bool) error {
	dir := path.Join(c.StackerDir, "imports", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	existing, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "couldn't read existing directory")
	}

	for _, i := range imports {
		name, err := acquireUrl(c, i, dir, progress)
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
