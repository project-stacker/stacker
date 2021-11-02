package overlay

import (
	"github.com/anuvu/stacker/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	return Check(config)
}
