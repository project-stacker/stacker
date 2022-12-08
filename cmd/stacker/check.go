package main

import (
	"io/fs"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/pkg/xattr"
	"github.com/urfave/cli"
	"stackerbuild.io/stacker/pkg/overlay"
)

var checkCmd = cli.Command{
	Name:   "check",
	Usage:  "checks that all runtime required things (like kernel features) are present",
	Action: doCheck,
}

func doCheck(ctx *cli.Context) error {
	if err := os.MkdirAll(config.RootFSDir, 0700); err != nil {
		return errors.Wrapf(err, "couldn't create rootfs dir for testing")
	}

	if e := verifyNewUIDMap(ctx); e != nil {
		return e
	}

	switch config.StorageType {
	case "overlay":
		return overlay.Check(config)
	default:
		return errors.Errorf("invalid storage type %v", config.StorageType)
	}
}

func verifyNewUIDMap(ctx *cli.Context) error {
	binFile, err := exec.LookPath("newuidmap")
	if err != nil {
		return errors.Wrapf(err, "newuidmap not found in path")
	}

	fileInfo, err := os.Stat(binFile)
	if err != nil {
		return errors.Wrapf(err, "couldn't stat file: %s", binFile)
	}

	if fileInfo.Mode()&0111 == 0 {
		return errors.Errorf("%s is not executable", binFile)
	}

	if fileInfo.Mode()&fs.ModeSetuid != 0 {
		// setuid-root is present, we are good!
		return nil
	}

	if e := checkForCap(binFile, "security.capability"); e != nil {
		return errors.Wrapf(e, "%s does not have either setuid-root or security caps", binFile)
	}

	return nil
}

func checkForCap(f string, cap string) error {
	caps, e := xattr.List(f)
	if e != nil {
		return errors.Errorf("could not read caps of %s", f)
	}

	for _, fcap := range caps {
		if fcap == cap {
			return nil
		}
	}

	return errors.Errorf("no security cap")
}
