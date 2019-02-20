package main

import (
	"context"
	"fmt"
	"time"

	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/mutate"
	"github.com/openSUSE/umoci/oci/layer"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
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

func doUnpack(ctx *cli.Context) error {
	fmt.Println("oci-dir", ctx.String("oci-dir"))
	oci, err := umoci.OpenLayout(ctx.String("oci-dir"))
	if err != nil {
		return err
	}

	opts := layer.MapOptions{KeepDirlinks: true}
	return umoci.Unpack(oci, ctx.String("tag"), ctx.String("bundle-path"), opts)
}

func doRepack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(ctx.String("oci-dir"))
	if err != nil {
		return err
	}

	bundlePath := ctx.String("bundle-path")
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

	return umoci.Repack(oci, ctx.String("tag"), bundlePath, meta, history, nil, true, mutator)
}
