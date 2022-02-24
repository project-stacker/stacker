package overlay

import (
	"github.com/project-stacker/stacker/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	return Check(config)
}
