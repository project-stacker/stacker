package main

import (
	"os"
	"regexp"

	"github.com/anuvu/stacker"
	"github.com/anuvu/stacker/lib"

	"github.com/urfave/cli"
)

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
			Value: "stacker.yaml",
		},
		cli.StringFlag{
			Name:  "search-dir, d",
			Usage: "directory under which to search for stackerfiles to build",
			Value: ".",
		})
}

func beforeRecursiveBuild(ctx *cli.Context) error {

	// Validate arguments which are common to build
	err := beforeBuild(ctx)
	if err != nil {
		return err
	}

	// Use the current working directory if base search directory is "."
	if ctx.String("search-dir") == "." {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		err = ctx.Set("search-dir", wd)
		if err != nil {
			return err
		}
	}

	// Ensure the base search directory exists
	if _, err := os.Lstat(ctx.String("search-dir")); err != nil {
		return err
	}

	// Ensure the stacker-file-pattern variable compiles as a regex
	if _, err := regexp.Compile(ctx.String("stacker-file-pattern")); err != nil {
		return err
	}

	return nil
}

func doRecursiveBuild(ctx *cli.Context) error {
	args := newBuildArgs(ctx)

	stackerFiles, err := lib.FindFiles(ctx.String("search-dir"), ctx.String("stacker-file-pattern"))
	if err != nil {
		return err
	}

	builder := stacker.NewBuilder(&args)
	return builder.BuildMultiple(stackerFiles)
}
