package main

import (
	"fmt"

	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
)

var buildCmd = cli.Command{
	Name:   "build",
	Usage:  "builds a new OCI image from a stacker yaml file",
	Action: doBuild,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "leave-unladen",
			Usage: "leave the built rootfs mount after image building",
		},
		cli.StringFlag{
			Name:  "stacker-file, f",
			Usage: "the input stackerfile",
			Value: "stacker.yaml",
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "don't use the previous build cache",
		},
		cli.StringSliceFlag{
			Name:  "substitute",
			Usage: "variable substitution in stackerfiles, FOO=bar format",
		},
		cli.StringFlag{
			Name:  "on-run-failure",
			Usage: "command to run inside container if run fails (useful for inspection)",
		},
		cli.BoolFlag{
			Name:  "shell-fail",
			Usage: "exec /bin/sh inside the container if run fails (alias for --on-run-failure=/bin/sh)",
		},
		cli.BoolFlag{
			Name:  "apply-consider-timestamps",
			Usage: "for apply layer merging, fail if timestamps on files don't match",
		},
		cli.StringFlag{
			Name:  "layer-type",
			Usage: "set the output layer type (supported values: tar, squashfs)",
			Value: "tar",
		},
	},
	Before: beforeBuild,
}

func beforeBuild(ctx *cli.Context) error {
	if ctx.Bool("shell-fail") {
		askedFor := ctx.String("on-run-failure")
		if askedFor != "" && askedFor != "/bin/sh" {
			return fmt.Errorf("--shell-fail is incompatible with --on-run-failure=%s", askedFor)
		}
		err := ctx.Set("on-run-failure", "/bin/sh")
		if err != nil {
			return err
		}
	}

	switch ctx.String("layer-type") {
	case "tar":
		break
	case "squashfs":
		fmt.Println("squashfs support is experimental")
		break
	default:
		return fmt.Errorf("unknown layer type: %s", ctx.String("layer-type"))
	}

	return nil
}

func doBuild(ctx *cli.Context) error {
	args := stacker.BuildArgs{
		Config:                  config,
		LeaveUnladen:            ctx.Bool("leave-unladen"),
		StackerFile:             ctx.String("stacker-file"),
		NoCache:                 ctx.Bool("no-cache"),
		Substitute:              ctx.StringSlice("substitute"),
		OnRunFailure:            ctx.String("on-run-failure"),
		ApplyConsiderTimestamps: ctx.Bool("apply-consider-timestamps"),
		LayerType:               ctx.String("layer-type"),
	}

	return stacker.Build(&args)
}
