package main

import (
	"github.com/urfave/cli"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/stacker"
)

const stackerFilePathRegex = "\\/stacker.yaml$"

var recursiveBuildCmd = cli.Command{
	Name:   "recursive-build",
	Usage:  "finds stacker yaml files under a directory and builds all OCI layers they define",
	Action: doRecursiveBuild,
	Flags:  initRecursiveBuildFlags(),
	Before: beforeRecursiveBuild,
}

func initRecursiveBuildFlags() []cli.Flag {
	return append(
		initCommonBuildFlags(),
		cli.StringFlag{
			Name:  "stacker-file-pattern, p",
			Usage: "regex pattern to use when searching for stackerfile paths",
			Value: stackerFilePathRegex,
		},
		cli.StringFlag{
			Name:  "search-dir, d",
			Usage: "directory under which to search for stackerfiles to build",
			Value: ".",
		})
}

func beforeRecursiveBuild(ctx *cli.Context) error {

	// Validate build failure arguments
	err := validateBuildFailureFlags(ctx)
	if err != nil {
		return err
	}

	// Validate layer type
	err = validateLayerTypeFlags(ctx)
	if err != nil {
		return err
	}

	// Validate search arguments
	err = validateFileSearchFlags(ctx)
	if err != nil {
		return err
	}

	return nil
}

func doRecursiveBuild(ctx *cli.Context) error {
	args, err := newBuildArgs(ctx)
	if err != nil {
		return err
	}

	stackerFiles, err := lib.FindFiles(ctx.String("search-dir"), ctx.String("stacker-file-pattern"))
	if err != nil {
		return err
	}

	builder := stacker.NewBuilder(&args)
	return builder.BuildMultiple(stackerFiles)
}
