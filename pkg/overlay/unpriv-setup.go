package overlay

import (
	"stackerbuild.io/stacker/pkg/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	return Check(config)
}
