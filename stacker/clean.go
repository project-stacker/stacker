package main

import (
	"os"
	"path"
	"syscall"

	"github.com/urfave/cli"
)

var cleanCmd = cli.Command{
	Name:   "clean",
	Usage:  "cleans up after a `stacker build`",
	Action: doClean,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "all",
			Usage: "clean imports, not just build products",
		},
	},
}

func doClean(ctx *cli.Context) error {
	// Explicitly don't check errors. We want to do what we can to just
	// clean everything up.
	syscall.Unmount(config.RootFSDir, syscall.MNT_DETACH)
	os.RemoveAll(config.RootFSDir)
	os.RemoveAll(config.OCIDir)

	if !ctx.Bool("all") {
		os.RemoveAll(path.Join(config.StackerDir, "logs"))
		os.Remove(path.Join(config.StackerDir, "build.cache"))
		os.Remove(path.Join(config.StackerDir, "btrfs.loop"))
	} else {
		os.RemoveAll(config.StackerDir)
	}

	return nil
}
