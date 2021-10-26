package main

import (
	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
)

var gcCmd = cli.Command{
	Name:   "gc",
	Usage:  "gc unused OCI imports/outputs and btrfs snapshots",
	Action: doGC,
}

func doGC(ctx *cli.Context) error {
	s, locks, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer s.Detach()
	defer locks.Unlock()
	return s.GC()
}
