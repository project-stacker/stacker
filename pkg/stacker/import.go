package stacker

import (
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"

	"github.com/pkg/errors"
	"github.com/udhos/equalfile"
	"github.com/vbatts/go-mtree"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
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

func importFile(imp string, cacheDir string, hash string, idest string, mode *fs.FileMode, uid, gid int) (string, error) {
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
		if idest != "" && idest[len(idest)-1:] != "/" {
			dest = path.Join(cacheDir, path.Base(idest))
		}
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
			if err := lib.FileCopy(dest, imp, mode, uid, gid); err != nil {
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
				return "", errors.Wrapf(err, "couldn't remove missing import %s", path.Join(cacheDir, path.Base(imp), d.Path()))
			}
		case mtree.Modified:
			fallthrough
		case mtree.Extra:
			srcpath := path.Join(imp, d.Path())
			destpath := path.Join(cacheDir, path.Base(imp), d.Path())

			if d.New().IsDir() {
				fi, err := os.Lstat(destpath)
				if err != nil {
					if !os.IsNotExist(err) {
						return "", errors.WithStack(err)
					}
				} else if !fi.IsDir() {
					/*
					 * if the thing changed from a file to
					 * a directory, we should delete it.
					 * Note that we *only* want to do the
					 * delete in this case, but not if it
					 * was previously a dir, since we
					 * iterate over diffs in an arbitrary
					 * order, so we may have already
					 * imported stuff below this, resulting
					 * in an incorrect delete.
					 */
					err = os.Remove(destpath)
					if err != nil {
						return "", errors.WithStack(err)
					}
				}

				err = errors.WithStack(os.MkdirAll(destpath, 0755))
				if err != nil {
					return "", err
				}
			} else {
				err = errors.WithStack(os.MkdirAll(path.Dir(destpath), 0755))
				if err != nil {
					return "", err
				}
				err = lib.FileCopy(destpath, srcpath, mode, uid, gid)
			}
			if err != nil {
				return "", err
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

func acquireUrl(c types.StackerConfig, storage types.Storage, i string, cache string, expectedHash string,
	idest string, mode *fs.FileMode, uid, gid int, progress bool,
) (string, error) {
	url, err := types.NewDockerishUrl(i)
	if err != nil {
		return "", err
	}

	// validate the given hash
	if err = validateHash(expectedHash); err != nil {
		return "", err
	}

	// It's just a path, let's copy it to .stacker.
	if url.Scheme == "" {
		return importFile(i, cache, expectedHash, idest, mode, uid, gid)
	} else if url.Scheme == "http" || url.Scheme == "https" {
		// otherwise, we need to download it
		// first verify the hashes
		remoteHash, remoteSize, err := getHttpFileInfo(i)
		if err != nil {
			// Needed for "working offline"
			// See https://stackerbuild.io/stacker/issues/44
			log.Infof("cannot obtain file info of %s", i)
		}
		log.Debugf("Remote file: hash: %s length: %s", remoteHash, remoteSize)
		// verify if the given hash from stackerfile matches the remote one.
		if len(expectedHash) > 0 && len(remoteHash) > 0 && strings.ToLower(expectedHash) != remoteHash {
			return "", errors.Errorf("The requested hash of %s import is different than the actual hash: %s != %s",
				i, expectedHash, remoteHash)
		}
		return Download(cache, i, progress, expectedHash, remoteHash, remoteSize, idest, mode, uid, gid)
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
		err = Grab(c, storage, snap, url.Path, cache, mode, uid, gid)
		if err != nil {
			return "", err
		}

		err = verifyImportFileHash(p, expectedHash)
		if err != nil {
			return "", err
		}

		return p, nil
	}

	return "", errors.Errorf("unsupported url scheme %s", i)
}

func CleanImportsDir(c types.StackerConfig, name string, imports types.Imports, cache *BuildCache) error {
	// remove all copied imports
	dir := path.Join(c.StackerDir, "imports-copy", name)
	_ = os.RemoveAll(dir)

	dir = path.Join(c.StackerDir, "imports", name)

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

// Import files from different sources to an ephemeral or permanent destination.
func Import(c types.StackerConfig, storage types.Storage, name string, imports types.Imports, overlayDirs *types.OverlayDirs, progress bool) error {
	dir := path.Join(c.StackerDir, "imports", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	cpdir := path.Join(c.StackerDir, "imports-copy", name)

	if err := os.MkdirAll(cpdir, 0755); err != nil {
		return err
	}

	existing, err := os.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "couldn't read existing directory")
	}

	for _, i := range imports {
		cache := dir

		// if "import" directives has a "dest", then convert them into overlay_dir entries
		if i.Dest != "" {
			tmpdir, err := os.MkdirTemp(cpdir, "")
			if err != nil {
				return errors.Wrapf(err, "couldn't create temp import copy directory")
			}

			dest := i.Dest

			if i.Dest[len(i.Dest)-1:] != "/" {
				dest = path.Dir(i.Dest)
			}

			ovl := types.OverlayDir{Source: tmpdir, Dest: dest}
			*overlayDirs = append(*overlayDirs, ovl)

			cache = tmpdir
		}

		name, err := acquireUrl(c, storage, i.Path, cache, i.Hash, i.Dest, i.Mode, i.Uid, i.Gid, progress)
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
