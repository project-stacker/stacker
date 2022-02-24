package btrfs

import (
	"path"

	"github.com/project-stacker/stacker/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	size := int64(100 * 1024 * 1024 * 1024)
	return MakeLoopbackBtrfs(path.Join(config.StackerDir, "btrfs.loop"), size, uid, gid, config.RootFSDir)
}
