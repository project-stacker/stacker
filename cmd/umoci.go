package main

import (
	"context"
	"os"
	"time"

	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/mutate"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var umociCmd = cli.Command{
	Name:   "umoci",
	Hidden: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name: "oci-dir",
		},
		cli.StringFlag{
			Name: "bundle-path",
		},
		cli.StringFlag{
			Name: "tag",
		},
	},
	Subcommands: []cli.Command{
		cli.Command{
			Name:   "init",
			Action: doInit,
		},
		cli.Command{
			Name:   "unpack",
			Action: doUnpack,
		},
		cli.Command{
			Name:   "repack",
			Action: doRepack,
			Flags: []cli.Flag{
				cli.Uint64Flag{
					Name: "max-layer-size",
				},
			},
		},
	},
}

func doInit(ctx *cli.Context) error {
	name := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-dir")
	bundlePath := ctx.GlobalString("bundle-path")
	var oci casext.Engine
	var err error

	if _, statErr := os.Stat(ociDir); statErr != nil {
		oci, err = umoci.CreateLayout(ociDir)
	} else {
		oci, err = umoci.OpenLayout(ociDir)
	}
	if err != nil {
		return errors.Wrapf(err, "Failed creating layout for %s", ociDir)
	}
	err = umoci.NewImage(oci, name)
	if err != nil {
		return errors.Wrapf(err, "umoci tag creation failed")
	}

	opts := layer.MapOptions{KeepDirlinks: true}
	err = umoci.Unpack(oci, name, bundlePath, opts)
	if err != nil {
		return errors.Wrapf(err, "umoci unpack failed for %s into %s", name, bundlePath)
	}

	return nil
}

func doUnpack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(ctx.GlobalString("oci-dir"))
	if err != nil {
		return err
	}

	opts := layer.MapOptions{KeepDirlinks: true}
	return umoci.Unpack(oci, ctx.GlobalString("tag"), ctx.GlobalString("bundle-path"), opts)
}

func doRepack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(ctx.GlobalString("oci-dir"))
	if err != nil {
		return err
	}

	bundlePath := ctx.GlobalString("bundle-path")
	meta, err := umoci.ReadBundleMeta(bundlePath)
	if err != nil {
		return err
	}

	mutator, err := mutate.New(oci, meta.From)
	if err != nil {
		return err
	}

	imageMeta, err := mutator.Meta(context.Background())
	if err != nil {
		return err
	}

	now := time.Now()
	history := &ispec.History{
		Author:     imageMeta.Author,
		Created:    &now,
		CreatedBy:  "stacker umoci repack",
		EmptyLayer: false,
	}

	return umoci.Repack(oci, ctx.GlobalString("tag"), bundlePath, meta, history, nil, true, mutator)
}
