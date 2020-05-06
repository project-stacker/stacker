package main

import (
	"fmt"
	"os"
	"path"

	"github.com/anuvu/stacker"
	"github.com/anuvu/stacker/log"
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
	stacker.CleanRoots(config)
	os.RemoveAll(config.RootFSDir)
	os.RemoveAll(config.OCIDir)

	fail := false

	if !ctx.Bool("all") {
		if err := os.Remove(path.Join(config.StackerDir, "build.cache")); err != nil {
			if !os.IsNotExist(err) {
				log.Infof("error deleting logs dir: %v", err)
				fail = true
			}
		}
		if err := os.Remove(path.Join(config.StackerDir, "btrfs.loop")); err != nil {
			if !os.IsNotExist(err) {
				log.Infof("error deleting btrfs loop: %v", err)
				fail = true
			}
		}
	} else {
		if err := os.RemoveAll(config.StackerDir); err != nil {
			if !os.IsNotExist(err) {
				log.Infof("error deleting stacker dir: %v", err)
				fail = true
			}
		}
	}

	if fail {
		return fmt.Errorf("cleaning failed")
	}

	return nil
}
