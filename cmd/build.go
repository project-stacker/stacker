package main

import (
	"fmt"

	"github.com/urfave/cli"

	"github.com/anuvu/stacker"
)

var buildCmd = cli.Command{
	Name:   "build",
	Usage:  "builds a new OCI image from a stacker yaml file",
	Action: doBuild,
	Flags:  initBuildFlags(),
	Before: beforeBuild,
}

func initBuildFlags() []cli.Flag {
	return append(
		initCommonBuildFlags(),
		cli.StringFlag{
			Name:  "stacker-file, f",
			Usage: "the input stackerfile",
			Value: "stacker.yaml",
		})
}

func initCommonBuildFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:  "leave-unladen",
			Usage: "leave the built rootfs mount after image building",
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
		cli.BoolFlag{
			Name:  "order-only",
			Usage: "show the build order without running the actual build",
		},
	}
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

func newBuildArgs(ctx *cli.Context) stacker.BuildArgs {
	return stacker.BuildArgs{
		Config:                  config,
		LeaveUnladen:            ctx.Bool("leave-unladen"),
		NoCache:                 ctx.Bool("no-cache"),
		Substitute:              ctx.StringSlice("substitute"),
		OnRunFailure:            ctx.String("on-run-failure"),
		ApplyConsiderTimestamps: ctx.Bool("apply-consider-timestamps"),
		LayerType:               ctx.String("layer-type"),
		OrderOnly:               ctx.Bool("order-only"),
		Debug:                   debug,
	}
}

func doBuild(ctx *cli.Context) error {
	args := newBuildArgs(ctx)

	builder := stacker.NewBuilder(&args)
	return builder.BuildMultiple([]string{ctx.String("stacker-file")})
}
