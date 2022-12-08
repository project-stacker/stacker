package main

import (
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/squashfs"
	"stackerbuild.io/stacker/pkg/stacker"
	"stackerbuild.io/stacker/pkg/types"
)

var publishCmd = cli.Command{
	Name:   "publish",
	Usage:  "publishes OCI images previously built from one or more stacker yaml files",
	Action: doPublish,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "stacker-file, f",
			Usage: "the input stackerfile",
			Value: "stacker.yaml",
		},
		cli.StringFlag{
			Name:  "stacker-file-pattern, p",
			Usage: "regex pattern to use when searching for stackerfile paths",
			Value: stackerFilePathRegex,
		},
		cli.StringFlag{
			Name:  "search-dir, d",
			Usage: "directory under which to search for stackerfiles to publish",
		},
		cli.StringFlag{
			Name:  "url",
			Usage: "url where to publish the OCI images",
		},
		cli.StringFlag{
			Name:  "username",
			Usage: "username for the registry where the OCI images are published",
		},
		cli.StringFlag{
			Name:  "password",
			Usage: "password for the registry where the OCI images are published",
		},
		cli.BoolFlag{
			Name:  "skip-tls",
			Usage: "skip tls verify on upstream registry",
		},
		cli.StringSliceFlag{
			Name:  "tag",
			Usage: "tag to be used when publishing",
		},
		cli.StringSliceFlag{
			Name:  "substitute",
			Usage: "variable substitution in stackerfiles, FOO=bar format",
		},
		cli.BoolFlag{
			Name:  "show-only",
			Usage: "show the images to be published without actually publishing them",
		},
		cli.BoolFlag{
			Name:  "force",
			Usage: "force publishing the images present in the OCI layout even if they should be rebuilt",
		},
		cli.StringSliceFlag{
			Name:  "layer-type",
			Usage: "set the output layer type (supported values: tar, squashfs); can be supplied multiple times",
			Value: &cli.StringSlice{"tar"},
		},
	},
	Before: beforePublish,
}

func beforePublish(ctx *cli.Context) error {

	// Check if search-dir and stacker-file-pattern are in use or if we should
	// fallback to stacker-file, in which case we don't need this validation
	if len(ctx.String("search-dir")) != 0 {
		// Validate search arguments
		err := validateFileSearchFlags(ctx)
		if err != nil {
			return err
		}
	}

	username := ctx.String("username")
	password := ctx.String("password")
	if (username == "") != (password == "") {
		return errors.Errorf("supply both username and password, or none of them, current values: '%s' '%s'",
			username,
			password)
	}

	if len(ctx.String("url")) == 0 {
		return errors.Errorf("--url is a mandatory argument for publishing")
	}

	return nil
}

func doPublish(ctx *cli.Context) error {
	verity := squashfs.VerityMetadata(!ctx.Bool("no-squashfs-verity"))
	layerTypes, err := types.NewLayerTypes(ctx.StringSlice("layer-type"), verity)
	if err != nil {
		return err
	}

	args := stacker.PublishArgs{
		Config:     config,
		ShowOnly:   ctx.Bool("show-only"),
		Substitute: ctx.StringSlice("substitute"),
		Tags:       ctx.StringSlice("tag"),
		Url:        ctx.String("url"),
		Username:   ctx.String("username"),
		Password:   ctx.String("password"),
		Force:      ctx.Bool("force"),
		Progress:   shouldShowProgress(ctx),
		SkipTLS:    ctx.Bool("skip-tls"),
		LayerTypes: layerTypes,
	}

	var stackerFiles []string
	if len(ctx.String("search-dir")) > 0 {
		// Need to search for all the paths matching the stacker-file regex under search-dir
		stackerFiles, err = lib.FindFiles(ctx.String("search-dir"), ctx.String("stacker-file-pattern"))
	} else {
		stackerFiles = []string{ctx.String("stacker-file")}
	}

	if err != nil {
		return err
	}

	publisher := stacker.NewPublisher(&args)
	return publisher.PublishMultiple(stackerFiles)
}
