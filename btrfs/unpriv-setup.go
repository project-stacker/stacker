package btrfs

import (
	"os"
	"path"
	"path/filepath"

	"github.com/anuvu/stacker/types"
)

func recursiveChown(dir string, uid int, gid int) error {
	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return os.Chown(p, uid, gid)
	})
}

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	size := int64(100 * 1024 * 1024 * 1024)
	err := MakeLoopbackBtrfs(path.Join(config.StackerDir, "btrfs.loop"), size, uid, gid, config.RootFSDir)
	if err != nil {
		return err
	}

	err = recursiveChown(config.StackerDir, uid, gid)
	if err != nil {
		return err
	}

	return recursiveChown(config.RootFSDir, uid, gid)
}
