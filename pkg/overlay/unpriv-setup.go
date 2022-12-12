package overlay

import (
	"stackerbuild.io/stacker/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	return Check(config)
}
