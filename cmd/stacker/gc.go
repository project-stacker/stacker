package main

import (
	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker/pkg/stacker"
)

var gcCmd = cli.Command{
	Name:   "gc",
	Usage:  "gc unused OCI imports/outputs snapshots",
	Action: doGC,
}

func doGC(ctx *cli.Context) error {
	s, locks, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer locks.Unlock()
	return s.GC()
}
