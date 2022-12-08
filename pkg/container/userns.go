package container

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	stackeridmap "stackerbuild.io/stacker/pkg/container/idmap"
	embed_exec "stackerbuild.io/stacker/pkg/embed-exec"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

// A wrapper which executes userCmd in a namespace if stacker has not already
// entered a namespace.  If it has (STACKER_REAL_UID is set), then it will simply
// exeucte the userCmd.
//
// If the real uid is 0, then use 'nsexec', otherwise use 'usernsexec'.
func MaybeRunInNamespace(config types.StackerConfig, userCmd []string) error {
	const envName = "STACKER_REAL_UID"
	env := os.Environ()
	realuid := os.Getenv(envName)
	if realuid != "" {
		cmd := exec.Command(userCmd[0], userCmd[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = env
		return errors.WithStack(cmd.Run())
	}

	euid := os.Geteuid()
	env = append(env, fmt.Sprintf("%s=%d", envName, euid))

	args := []string{}
	if euid == 0 {
		args = append(args, "nsexec")
	} else {
		idmapSet, err := stackeridmap.ResolveCurrentIdmapSet()
		if err != nil {
			return err
		}

		if idmapSet == nil {
			return errors.Errorf("no idmap and not root, can't run %v", userCmd)
		}

		args = append(args, "usernsexec")
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

	}

	args = append(args, "--")
	args = append(args, userCmd...)

	log.Debugf("%s-ing %v", args[0], args[1:])
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
	cmd.Env = env
	return errors.WithStack(cmd.Run())
}
