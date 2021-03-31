package stacker

import (
	"fmt"
	"github.com/opencontainers/go-digest"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

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

func chmodParentAndRemove(destpath string) (func(), error) {
	// in the unpriv case, the dir might be -w (centos distributes its
	// /root this way), and non-real root can't delete stuff that's -w,
	// even if it is the owner. so let's chmod +w .. and try again.
	dir := path.Dir(destpath)
	orig, err := os.Stat(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't chmod +w ..")
	}

	err = os.Chmod(dir, 0700)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't chmod +w ..")
	}
	return func() { os.Chmod(dir, orig.Mode()) }, os.RemoveAll(destpath)
}

func verifyImportFileHash(imp string, hash string) error {
	if len(hash) == 0 {
		return nil
	}
	actualHash, err := lib.HashFile(imp, false)
	if err != nil {
		return err
	}

	actualHash = strings.TrimPrefix(actualHash, "sha256:")
	if actualHash != strings.ToLower(hash) {
		return errors.Errorf("The requested hash of %s import is different than the actual hash: %s != %s",
			imp, hash, actualHash)
	}

	return nil
}

func importFile(imp string, cacheDir string, hash string) (string, error) {
	e1, err := os.Lstat(imp)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't stat import %s", imp)
	}

	if !e1.IsDir() {
		err := verifyImportFileHash(imp, hash)
		if err != nil {
			return "", err
		}
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
			p := path.Join(cacheDir, path.Base(imp), d.Path())
			err := os.RemoveAll(p)
			if err != nil {
				if os.IsPermission(err) {
					var cleanup func()
					cleanup, err = chmodParentAndRemove(p)
					if cleanup != nil {
						defer cleanup()
					}
				}

				if err != nil {
					return "", errors.Wrapf(err, "couldn't remove missing import %s", path.Join(cacheDir, path.Base(imp), d.Path()))
				}
			}
		case mtree.Modified:
			fallthrough
		case mtree.Extra:
			srcpath := path.Join(imp, d.Path())
			destpath := path.Join(cacheDir, path.Base(imp), d.Path())

			err = os.RemoveAll(destpath)
			if err != nil && !os.IsNotExist(err) {
				if os.IsPermission(err) {
					var cleanup func()
					cleanup, err = chmodParentAndRemove(destpath)
					if cleanup != nil {
						defer cleanup()
					}
				}

				if err != nil {
					return "", errors.Wrapf(err, "couldn't remove to replace import %s", destpath)
				}
			}

			sdirinfo, err := os.Lstat(path.Dir(srcpath))
			if err != nil {
				return "", errors.Wrapf(err, "couldn't stat source import %s", srcpath)
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

func validateHash(hash string) error {
	if len(hash) > 0 {
		log.Debugf("hash: %#v", hash)
		// Validate given hash from stackerfile
		validator := digest.Algorithm("sha256")
		if err := validator.Validate(strings.ToLower(hash)); err != nil {
			return errors.Wrapf(err, "Given hash %s is not valid", hash)
		}
	}
	return nil
}

func acquireUrl(c types.StackerConfig, storage types.Storage, i string, cache string, progress bool, hash string) (string, error) {
	url, err := types.NewDockerishUrl(i)
	if err != nil {
		return "", err
	}

	// validate the given hash
	if err = validateHash(hash); err != nil {
		return "", err
	}

	// It's just a path, let's copy it to .stacker.
	if url.Scheme == "" {
		return importFile(i, cache, hash)
	} else if url.Scheme == "http" || url.Scheme == "https" {
		// otherwise, we need to download it
		// first verify the hashes
		remoteHash, remoteSize, err := getHttpFileInfo(i)
		if err != nil {
			// Needed for "working offline"
			// See https://github.com/anuvu/stacker/issues/44
			log.Infof("cannot obtain file info of %s", i)
		}
		log.Debugf("Remote file: hash: %s length: %s", remoteHash, remoteSize)
		// verify if the given hash from stackerfile matches the remote one.
		if len(hash) > 0 && len(remoteHash) > 0 && strings.ToLower(hash) != remoteHash {
			return "", errors.Errorf("The requested hash of %s import is different than the actual hash: %s != %s",
				i, hash, remoteHash)
		}
		return Download(cache, i, progress, remoteHash, remoteSize)
	} else if url.Scheme == "stacker" {
		// we always Grab() things from stacker://, because we need to
		// mount the container's rootfs to get them and don't
		// necessarily have a good way to do that. so this i/o is
		// always done.
		p := path.Join(cache, path.Base(url.Path))
		snap, cleanup, err := storage.TemporaryWritableSnapshot(url.Host)
		if err != nil {
			return "", err
		}
		defer cleanup()
		err = Grab(c, storage, snap, url.Path, cache)
		if err != nil {
			return "", err
		}
		err = verifyImportFileHash(p, hash)
		if err != nil {
			return "", err
		}

		return p, nil
	}

	return "", errors.Errorf("unsupported url scheme %s", i)
}

func CleanImportsDir(c types.StackerConfig, name string, imports types.Imports, cache *BuildCache) error {
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
			if path.Base(cached) == path.Base(i.Path) && cached != i.Path {
				log.Infof("%s url changed to %s, pruning cache", cached, i.Path)
				err := os.RemoveAll(path.Join(dir, path.Base(i.Path)))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func Import(c types.StackerConfig, storage types.Storage, name string, imports types.Imports, progress bool) error {
	dir := path.Join(c.StackerDir, "imports", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	existing, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "couldn't read existing directory")
	}

	for _, i := range imports {
		name, err := acquireUrl(c, storage, i.Path, dir, progress, i.Hash)
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

// Copy imports to a container rootfs
func copyImportsInRootfs(name string, imports types.Imports, sc types.StackerConfig, storage types.Storage) error {
	cacheDir := path.Join(sc.StackerDir, "imports", name)
	for _, i := range imports {
		url, err := types.NewDockerishUrl(i.Path)
		if err != nil {
			return err
		}
		rootfsDir := "rootfs"
		source := path.Join(cacheDir, path.Base(i.Path))

		// logic for overlay storage
		// we need to mount host's import dir
		if storage.Name() == "overlay" {
			c, err := NewContainer(sc, storage, name)
			if err != nil {
				return err
			}
			defer c.Close()

			err = c.bindMount(cacheDir, "/stacker", "")
			if err != nil {
				return err
			}
			//defer os.Remove(path.Join(sc.RootFSDir, name, "rootfs", "stacker"))
			// mkdir destination if it doesn't exist
			dest := path.Join(i.Dest, path.Base(i.Path))
			if i.Dest != "/" {
				err = c.Execute(fmt.Sprintf("mkdir -p %s", i.Dest), nil)
				if err != nil {
					return err
				}
			}
			err = c.Execute(fmt.Sprintf("cp -r --preserve=all --no-preserve=ownership /stacker/%s %s", path.Base(i.Path), dest), nil)
			if err != nil {
				return err
			}
			continue
		}
		// logic for btrfs storage
		dest := path.Join(sc.RootFSDir, name, rootfsDir, i.Dest, path.Base(i.Path))
		// mkdir destination if it doesn't exist
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			if err = os.MkdirAll(dest, 0755); err != nil {
				return err
			}
		}

		if url.Scheme == "http" || url.Scheme == "https" {
			// import is file
			if err = lib.FileCopy(dest, source); err != nil {
				return err
			}
		} else {
			e1, err := os.Lstat(source)
			if err != nil {
				return err
			}
			if e1.IsDir() {
				if err = lib.DirCopy(dest, source); err != nil {
					return err
				}
			} else {
				if err = lib.FileCopy(dest, source); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
