package stacker

import (
	"os"

	"github.com/anuvu/stacker/btrfs"
	"github.com/anuvu/stacker/overlay"
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
)

func NewStorage(c types.StackerConfig) (types.Storage, *StackerLocks, error) {
	if err := os.MkdirAll(c.RootFSDir, 0755); err != nil {
		return nil, nil, err
	}

	switch c.StorageType {
	case "overlay":
		overlayOk, err := overlay.CanDoOverlay()
		if err != nil {
			return nil, nil, err
		}

		if !overlayOk {
			return nil, nil, errors.Errorf("can't do overlay operations but overlay backend requested")
		}
		s, err := overlay.NewOverlay(c)
		if err != nil {
			return nil, nil, err
		}
		locks, err := lock(c)
		if err != nil {
			return nil, nil, err
		}
		return s, locks, nil
	case "btrfs":
		isBtrfs, err := btrfs.DetectBtrfs(c.RootFSDir)
		if err != nil {
			return nil, nil, err
		}

		if !isBtrfs {
			s, err := btrfs.NewLoopback(c)
			if err != nil {
				return nil, nil, err
			}
			locks, err := lock(c)
			if err != nil {
				return nil, nil, err
			}
			return s, locks, nil
		}

		locks, err := lock(c)
		if err != nil {
			return nil, nil, err
		}
		return btrfs.NewExisting(c), locks, nil
	default:
		return nil, nil, errors.Errorf("unknown storage type %s", c.StorageType)
	}
}
