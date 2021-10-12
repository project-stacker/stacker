package stacker

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/anuvu/stacker/btrfs"
	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/overlay"
	"github.com/anuvu/stacker/storage"
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
)

var storageTypeFile = "storage.type"

// openStorage just opens a storage type, without doing any pre-existing
// storage checks
func openStorage(c types.StackerConfig, storageType string) (types.Storage, error) {
	switch storageType {
	case "overlay":
		err := overlay.CanDoOverlay(c)
		if err != nil {
			return nil, err
		}

		return overlay.NewOverlay(c)
	case "btrfs":
		isBtrfs, err := btrfs.DetectBtrfs(c.RootFSDir)
		if err != nil {
			log.Infof("error from DetectBtrfs %v", err)
			return nil, err
		}

		if !isBtrfs {
			log.Debugf("no btrfs detected, creating a loopback device")
			return btrfs.NewLoopback(c)
		}

		return btrfs.NewExisting(c), nil
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

	ents, err := ioutil.ReadDir(c.RootFSDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", errors.Wrapf(err, "couldn't read roots dir")
		}

		log.Debugf("no previous storage type detected")
		return "", nil
	}

	// nothing has been built, probably a new stacker, so let's use whatever we want
	if len(ents) == 0 {
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

// maybeSwitchStorage switches the storage type to the new user specified one
// if it differs from the old storage type that was used with this stacker
// instance before.
func maybeSwitchStorage(c types.StackerConfig) error {
	var storageType string
	content, err := ioutil.ReadFile(path.Join(c.StackerDir, storageTypeFile))
	if err != nil {
		// older versions of stacker didn't write this file
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't read storage type")
		}

		storageType, err = tryToDetectStorageType(c)
		if err != nil {
			return err
		}

		if storageType == "" {
			return nil
		}
	} else {
		storageType = string(content)
	}

	if storageType == c.StorageType {
		return nil
	}

	log.Infof("switching storage backend from %s to %s", storageType, c.StorageType)
	s, err := openStorage(c, storageType)
	if err != nil {
		return err
	}

	err = s.Clean()
	if err != nil {
		return err
	}

	err = os.Remove(c.CacheFile())
	// it's ok if it didn't exist, this is probably a new run of stacker
	if os.IsNotExist(err) {
		err = nil
	}

	return errors.Wrapf(err, "couldn't delete old cache")
}

func NewStorage(c types.StackerConfig) (types.Storage, error) {
	if err := os.MkdirAll(c.RootFSDir, 0755); err != nil {
		return nil, err
	}

	err := maybeSwitchStorage(c)
	if err != nil {
		return nil, err
	}

	log.Infof("using storage backend %s", c.StorageType)
	err = os.MkdirAll(c.StackerDir, 0755)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't make stacker dir")
	}

	err = os.MkdirAll(c.RootFSDir, 0755)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't make rootfs dir")
	}

	err = ioutil.WriteFile(path.Join(c.StackerDir, storageTypeFile), []byte(c.StorageType), 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't write storage type")
	}

	return openStorage(c, c.StorageType)
}

func UnprivSetup(c types.StackerConfig, username string, uid, gid int) error {
	err := storage.UidmapSetup(username, uid, gid)
	if err != nil {
		return err
	}

	switch c.StorageType {
	case "overlay":
		return overlay.UnprivSetup(c, uid, gid)
	case "btrfs":
		return btrfs.UnprivSetup(c, uid, gid)
	default:
		return errors.Errorf("unknown storage type %s", c.StorageType)
	}
}
