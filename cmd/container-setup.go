package main

import (
	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
)

var containerSetupCmd = cli.Command{
	Name:   "container-setup",
	Usage:  "set up (but don't run) any containers in the stacker file",
	Action: doContainerSetup,
	Flags:  initBuildFlags(),
	Before: beforeBuild,
}

func doContainerSetup(ctx *cli.Context) error {
	args := newBuildArgs(ctx)
	args.SetupOnly = true

	builder := stacker.NewBuilder(&args)
	return builder.BuildMultiple([]string{ctx.String("stacker-file")})
}
