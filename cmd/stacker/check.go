package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pkg/xattr"
	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/overlay"
	"stackerbuild.io/stacker/pkg/stacker"
)

var checkCmd = cli.Command{
	Name:   "check",
	Usage:  "checks that all runtime required things (like kernel features) are present",
	Action: doCheck,
}

func doCheck(ctx *cli.Context) error {

	kernel, err := stacker.KernelInfo()
	if err != nil {
		return errors.Wrapf(err, "couldn't get kernel info")
	}

	log.Infof("os/kernel: %s", kernel)

	if err := os.MkdirAll(config.RootFSDir, 0700); err != nil {
		return errors.Wrapf(err, "couldn't create rootfs dir for testing")
	}

	// internally there are many checks to avoid symlinks
	evalp, err := filepath.EvalSymlinks(config.RootFSDir)
	if err != nil {
		return errors.Wrapf(err, "%s: unable to evaluate path for symlinks", config.RootFSDir)
	}

	if evalp != config.RootFSDir {
		return errors.Errorf("%s: roots dir (--roots-dir) path uses symbolic links, use %q instead", config.RootFSDir, evalp)
	}

	// not all underlying filesystems are compatible
	fstype, err := stacker.MountInfo(config.RootFSDir)
	if err != nil {
		return errors.Wrapf(err, "%s: couldn't get fs type", config.RootFSDir)
	}

	log.Infof("%s %s", config.RootFSDir, fstype)

	if fstype == "NFS(6969)" {
		return errors.Errorf("roots dir (--roots-dir) path %s is not supported on NFS.", config.RootFSDir)
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
