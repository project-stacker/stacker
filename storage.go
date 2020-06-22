package stacker

import (
	"os"

	"github.com/anuvu/stacker/btrfs"
	"github.com/anuvu/stacker/types"
)

func NewStorage(c types.StackerConfig) (types.Storage, error) {
	if err := os.MkdirAll(c.RootFSDir, 0755); err != nil {
		return nil, err
	}

	isBtrfs, err := btrfs.DetectBtrfs(c.RootFSDir)
	if err != nil {
		return nil, err
	}

	if !isBtrfs {
		return btrfs.NewLoopback(c)
	}

	return btrfs.NewExisting(c), nil
}
