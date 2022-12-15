package main

import (
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/stacker"
)

var cleanCmd = cli.Command{
	Name:   "clean",
	Usage:  "cleans up after a `stacker build`",
	Action: doClean,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "all",
			Usage: "no-op; this used to do soemthing, and is left in for compatibility",
		},
	},
}

func doClean(ctx *cli.Context) error {
	fail := false

	if _, err := os.Stat(config.RootFSDir); !os.IsNotExist(err) {
		s, locks, err := stacker.NewStorage(config)
		if err != nil {
			return err
		}
		err = s.Clean()
		if err != nil {
			log.Infof("problem cleaning roots %v", err)
			fail = true
		}
		locks.Unlock()
	}

	if err := os.RemoveAll(config.OCIDir); err != nil {
		log.Infof("problem cleaning oci dir %v", err)
		fail = true
	}

	if err := os.RemoveAll(config.StackerDir); err != nil {
		if !os.IsNotExist(err) {
			log.Infof("error deleting stacker dir: %v", err)
			fail = true
		}
	}

	if fail {
		return errors.Errorf("cleaning failed")
	}

	return nil
}
