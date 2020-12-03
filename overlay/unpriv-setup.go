package overlay

import (
	"github.com/anuvu/stacker/types"
	"github.com/anuvu/stacker/container"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	return container.RunUmociSubcommand(config, []string{"check-overlay"})
}
