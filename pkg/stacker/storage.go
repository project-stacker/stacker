package stacker

import (
	"os"
	"path"

	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/overlay"
	"stackerbuild.io/stacker/pkg/storage"
	"stackerbuild.io/stacker/pkg/types"
)

var storageTypeFile = "storage.type"

// openStorage just opens a storage type, without doing any pre-existing
// storage checks
func openStorage(c types.StackerConfig, storageType string) (types.Storage, error) {
	switch storageType {
	case "overlay":
		err := overlay.Check(c)
		if err != nil {
			return nil, err
		}

		return overlay.NewOverlay(c)
	default:
		return nil, errors.Errorf("unknown storage type %s", storageType)
	}
}

// tryToDetectStorageType tries to detect the storage type for a configuration
// in the absence of the storage type file. there's a special value of "" when
// the roots directory didn't exist, meaning that there wasn't an old storage
// configuration and it's fine to start fresh.
func tryToDetectStorageType(c types.StackerConfig) (string, error) {
	// older versions of stacker left a few clues: if there's a
	// .stacker/btrfs.loop file, it was probably btrfs. if any of the roots
	// dirs have an overlay_metadata.json file in them, the backend was
	// overlay. if not, then it's probably btrfs.
	if _, err := os.Stat(path.Join(c.StackerDir, "btrfs.loop")); err == nil {
		log.Debugf("autodetected previous storage type of btrfs")
		return "btrfs", nil
	}

	ents, err := os.ReadDir(c.RootFSDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", errors.Wrapf(err, "couldn't read roots dir")
		}

		log.Debugf("no previous storage type detected")
		return "", nil
	}

	// nothing has been built (1 is for the lock file), probably a new
	// stacker, so let's use whatever we want
	if len(ents) <= 1 {
		log.Debugf("no previous storage type detected")
		return "", nil
	}

	for _, ent := range ents {
		if _, err := os.Stat(path.Join(c.RootFSDir, ent.Name(), "overlay")); err == nil {
			log.Debugf("detected some overlay layers, assuming previous storage type overlay")
			return "overlay", nil
		}
	}

	log.Debugf("no overlay layers detected, assuming previous storage type btrfs")
	return "btrfs", nil
}

// errorOnStorageTypeSwitch returns an error if there was a previous stacker
// run with a different storage type.
func errorOnStorageTypeSwitch(c types.StackerConfig) error {
	var storageType string
	content, err := os.ReadFile(path.Join(c.StackerDir, storageTypeFile))
	if err != nil {
		// older versions of stacker didn't write this file
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't read storage type")
		}

		storageType, err = tryToDetectStorageType(c)
		if err != nil {
			return err
		}

		// no previous storage is fine
		if storageType == "" {
			return nil
		}
	} else {
		storageType = string(content)
	}

	if storageType != c.StorageType {
		return errors.Errorf("previous storage type %s not compatible with requested storage %s", storageType, c.StorageType)
	}

	return nil

}

func NewStorage(c types.StackerConfig) (types.Storage, *StackerLocks, error) {
	if err := os.MkdirAll(c.RootFSDir, 0755); err != nil {
		return nil, nil, err
	}

	err := errorOnStorageTypeSwitch(c)
	if err != nil {
		return nil, nil, err
	}

	err = os.MkdirAll(c.StackerDir, 0755)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't make stacker dir")
	}

	err = os.MkdirAll(c.RootFSDir, 0755)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't make rootfs dir")
	}

	err = os.WriteFile(path.Join(c.StackerDir, storageTypeFile), []byte(c.StorageType), 0644)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't write storage type")
	}

	s, err := openStorage(c, c.StorageType)
	if err != nil {
		return nil, nil, err
	}

	// needed to attach the storage first, so we get the roots lock in the
	// right place. storage attachment mostly isn't racy (the kernel will
	// tell us EBUSY if we try to do the same btrfs loop mount twice, and
	// there is no attachment for overlay), so that's safe.
	locks, err := lock(c)
	if err != nil {
		return nil, nil, err
	}

	return s, locks, nil
}

func UnprivSetup(c types.StackerConfig, username string, uid, gid int) error {
	err := storage.UidmapSetup(username, uid, gid)
	if err != nil {
		return err
	}

	switch c.StorageType {
	case "overlay":
		return overlay.UnprivSetup(c, uid, gid)
	default:
		return errors.Errorf("unknown storage type %s", c.StorageType)
	}
}
