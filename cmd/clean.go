package main

import (
	"fmt"
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

	fail := false

	if !ctx.Bool("all") {
		if err := os.Remove(path.Join(config.StackerDir, "build.cache")); err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "error deleting logs dir: %v\n", err)
				fail = true
			}
		}
		if err := os.Remove(path.Join(config.StackerDir, "btrfs.loop")); err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "error deleting btrfs loop: %v\n", err)
				fail = true
			}
		}
	} else {
		if err := os.RemoveAll(config.StackerDir); err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "error deleting stacker dir: %v\n", err)
				fail = true
			}
		}
	}

	if fail {
		return fmt.Errorf("cleaning failed")
	}

	return nil
}
