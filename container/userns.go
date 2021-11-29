package container

import (
	"fmt"
	"os"
	"os/exec"

	stackeridmap "github.com/anuvu/stacker/container/idmap"
	"github.com/anuvu/stacker/embed-exec"
	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
)

// A wrapper which runs things in a userns if we're an unprivileged user with
// an idmap, or runs things on the host if we're root and don't.
func MaybeRunInUserns(config types.StackerConfig, userCmd []string) error {
	// TODO: we should try to use user namespaces when we're root as well.
	// For now we don't.
	if os.Geteuid() == 0 {
		log.Debugf("No uid mappings, running as root")
		cmd := exec.Command(userCmd[0], userCmd[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return errors.WithStack(cmd.Run())
	}

	idmapSet, err := stackeridmap.ResolveCurrentIdmapSet()
	if err != nil {
		return err
	}

	if idmapSet == nil {
		return errors.Errorf("no idmap and not root, can't run %v", userCmd)

	}

	args := []string{"usernsexec"}

	wroteU := false
	for _, idm := range idmapSet.Idmap {
		if idm.Isuid {
			if !wroteU {
				wroteU = true
				args = append(args, "u")
			}
			args = append(
				args,
				fmt.Sprintf("%d", idm.Nsid),
				fmt.Sprintf("%d", idm.Hostid),
				fmt.Sprintf("%d", idm.Maprange),
			)
		}
	}

	wroteG := false
	for _, idm := range idmapSet.Idmap {
		if idm.Isgid {
			if !wroteG {
				wroteG = true
				args = append(args, "g")
			}
			args = append(
				args,
				fmt.Sprintf("%d", idm.Nsid),
				fmt.Sprintf("%d", idm.Hostid),
				fmt.Sprintf("%d", idm.Maprange),
			)
		}
	}

	args = append(args, "--")
	args = append(args, userCmd...)

	log.Debugf("usernsexec-ing %v", args)
	cmd, cleanup, err := embed_exec.GetCommand(
		config.EmbeddedFS,
		"lxc-wrapper/lxc-wrapper",
		args...,
	)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return errors.WithStack(cmd.Run())
}
