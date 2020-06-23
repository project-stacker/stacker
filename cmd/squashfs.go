package main

import (
	"github.com/anuvu/stacker/squashfs"
	"github.com/opencontainers/umoci"
	"github.com/urfave/cli"
)

var squashfsCmd = cli.Command{
	Name:   "squashfs",
	Hidden: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name: "bundle-path",
		},
		cli.StringFlag{
			Name: "tag",
		},
		cli.StringFlag{
			Name: "author",
		},
	},
	Subcommands: []cli.Command{
		cli.Command{
			Name:   "repack",
			Action: squashfsRepack,
		},
	},
}

func squashfsRepack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}

	tag := ctx.GlobalString("tag")
	author := ctx.GlobalString("author")
	bundlePath := ctx.GlobalString("bundle-path")

	return squashfs.GenerateSquashfsLayer(tag, author, bundlePath, config.OCIDir, oci)
}
