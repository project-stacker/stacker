package overlay

import (
	"github.com/anuvu/stacker/container"
	"github.com/anuvu/stacker/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	return container.RunInternalGoSubcommand(config, []string{"check-overlay"})
}
