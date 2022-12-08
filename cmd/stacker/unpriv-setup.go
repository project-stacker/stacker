package main

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"stackerbuild.io/stacker/pkg/stacker"
)

var unprivSetupCmd = cli.Command{
	Name:   "unpriv-setup",
	Usage:  "do the necessary unprivileged setup for stacker build to work without root",
	Action: doUnprivSetup,
	Before: beforeUnprivSetup,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "uid",
			Usage: "the user to do setup for (defaults to $SUDO_UID from env)",
			Value: os.Getenv("SUDO_UID"),
		},
		cli.StringFlag{
			Name:  "gid",
			Usage: "the group to do setup for (defaults to $SUDO_GID from env)",
			Value: os.Getenv("SUDO_GID"),
		},
		cli.StringFlag{
			Name:  "username",
			Usage: "the username to do setup for (defaults to $SUDO_USER from env)",
			Value: os.Getenv("SUDO_USER"),
		},
	},
}

func beforeUnprivSetup(ctx *cli.Context) error {
	if ctx.String("uid") == "" {
		return errors.Errorf("please specify --uid or run unpriv-setup with sudo")
	}

	if ctx.String("gid") == "" {
		return errors.Errorf("please specify --gid or run unpriv-setup with sudo")
	}

	if ctx.String("username") == "" {
		return errors.Errorf("please specify --username or run unpriv-setup with sudo")
	}

	return nil
}

func recursiveChown(dir string, uid int, gid int) error {
	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return os.Chown(p, uid, gid)
	})
}

func doUnprivSetup(ctx *cli.Context) error {
	_, err := os.Stat(config.StackerDir)
	if err == nil {
		return errors.Errorf("stacker dir %s already exists, aborting setup", config.StackerDir)
	}

	uid, err := strconv.Atoi(ctx.String("uid"))
	if err != nil {
		return errors.Wrapf(err, "couldn't convert uid %s", ctx.String("uid"))
	}

	gid, err := strconv.Atoi(ctx.String("gid"))
	if err != nil {
		return errors.Wrapf(err, "couldn't convert gid %s", ctx.String("gid"))
	}

	err = os.MkdirAll(config.StackerDir, 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(config.RootFSDir, 0755)
	if err != nil {
		return err
	}

	username := ctx.String("username")

	err = stacker.UnprivSetup(config, username, uid, gid)
	if err != nil {
		return err
	}

	err = recursiveChown(config.StackerDir, uid, gid)
	if err != nil {
		return err
	}

	return recursiveChown(config.RootFSDir, uid, gid)
}
