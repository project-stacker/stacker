package main

import (
	"github.com/anuvu/stacker"
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
		cli.StringSliceFlag{
			Name:  "layer-type",
			Usage: "set the output layer type (supported values: tar, squashfs); can be supplied multiple times",
			Value: &cli.StringSlice{"tar"},
		},
		cli.BoolFlag{
			Name:  "require-hash",
			Usage: "require all remote imports to have a hash provided in stackerfiles",
		},
		cli.BoolFlag{
			Name:  "order-only",
			Usage: "show the build order without running the actual build",
		},
	}
}

func beforeBuild(ctx *cli.Context) error {
	if config.StorageType == "overlay" && ctx.Bool("leave-unladen") {
		return errors.Errorf("cannot use --storage-type=overlay and --leave-unladen together")
	}

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
	return nil
}

func newBuildArgs(ctx *cli.Context) (stacker.BuildArgs, error) {
	args := stacker.BuildArgs{
		Config:       config,
		LeaveUnladen: ctx.Bool("leave-unladen"),
		NoCache:      ctx.Bool("no-cache"),
		Substitute:   ctx.StringSlice("substitute"),
		OnRunFailure: ctx.String("on-run-failure"),
		OrderOnly:    ctx.Bool("order-only"),
		HashRequired: ctx.Bool("require-hash"),
		Progress:     shouldShowProgress(ctx),
	}
	var err error
	args.LayerTypes, err = types.NewLayerTypes(ctx.StringSlice("layer-type"))
	return args, err
}

func doBuild(ctx *cli.Context) error {
	args, err := newBuildArgs(ctx)
	if err != nil {
		return err
	}

	builder := stacker.NewBuilder(&args)
	return builder.BuildMultiple([]string{ctx.String("stacker-file")})
}
