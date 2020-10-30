package overlay

import (
	"github.com/anuvu/stacker/types"
)

func UnprivSetup(config types.StackerConfig, uid, gid int) error {
	// ideally we'd do something like the below,
	// return container.RunUmociSubcommand(config, []string{"check-overlay"})
	// in the target user's user namespace.
	// however, golang can't easily Setuid(), which makes that challenging.
	// so for now we just do nothing.
	return nil
}
