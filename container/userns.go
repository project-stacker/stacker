package container

import (
	"fmt"
	"os"
	"os/exec"

	stackeridmap "github.com/anuvu/stacker/container/idmap"
	"github.com/anuvu/stacker/log"
	"github.com/lxc/lxd/shared/idmap"
	"github.com/pkg/errors"
)

func runInUserns(idmapSet *idmap.IdmapSet, userCmd []string) (*exec.Cmd, error) {
	if idmapSet == nil {
		return nil, errors.Errorf("no subuids!")
	}

	args := []string{}
	for _, idm := range idmapSet.Idmap {
		var which string
		if idm.Isuid && idm.Isgid {
			which = "b"
		} else if idm.Isuid {
			which = "u"
		} else if idm.Isgid {
			which = "g"
		}

		m := fmt.Sprintf("%s:%d:%d:%d", which, idm.Nsid, idm.Hostid, idm.Maprange)
		args = append(args, "-m", m)
	}

	args = append(args, "--")
	args = append(args, userCmd...)

	cmd := exec.Command("lxc-usernsexec", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}

// A wrapper which runs things in a userns if we're an unprivileged user with
// an idmap, or runs things on the host if we're root and don't.
func MaybeRunInUserns(userCmd []string) (*exec.Cmd, error) {
	// TODO: we should try to use user namespaces when we're root as well.
	// For now we don't.
	if os.Geteuid() == 0 {
		log.Debugf("No uid mappings, running as root")
		cmd := exec.Command(userCmd[0], userCmd[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd, nil
	}

	idmapSet, err := stackeridmap.ResolveCurrentIdmapSet()
	if err != nil {
		return nil, err
	}

	if idmapSet == nil {
		if os.Geteuid() != 0 {
			return nil, errors.Errorf("no idmap and not root, can't run %v", userCmd)
		}

	}

	return runInUserns(idmapSet, userCmd)
}
