package main

import (
	"os"

	"github.com/pkg/errors"
	"github.com/project-stacker/stacker/overlay"
	"github.com/urfave/cli"
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

	switch config.StorageType {
	case "overlay":
		return overlay.Check(config)
	default:
		return errors.Errorf("invalid storage type %v", config.StorageType)
	}
}
