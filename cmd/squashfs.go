package main

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/anuvu/stacker"

	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/pkg/fseval"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
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
		cli.Command{
			Name:   "unpack",
			Action: squashfsUnpack,
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

	return stacker.GenerateSquashfsLayer(tag, author, bundlePath, config.OCIDir, oci)
}

func squashfsUnpack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}
	tag := ctx.GlobalString("tag")
	bundlePath := ctx.GlobalString("bundle-path")

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return err
	}

	for _, layer := range manifest.Layers {
		rootfs := path.Join(bundlePath, "rootfs")
		squashfsFile := path.Join(config.OCIDir, "blobs", "sha256", layer.Digest.Encoded())
		userCmd := []string{"unsquashfs", "-f", "-d", rootfs, squashfsFile}
		cmd := exec.Command(userCmd[0], userCmd[1:]...)
		cmd.Stdin = nil
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	dps, err := oci.ResolveReference(context.Background(), tag)
	if err != nil {
		return err
	}

	mtreeName := strings.Replace(dps[0].Descriptor().Digest.String(), ":", "_", 1)
	err = umoci.GenerateBundleManifest(mtreeName, bundlePath, fseval.DefaultFsEval)
	if err != nil {
		return err
	}

	err = umoci.WriteBundleMeta(bundlePath, umoci.Meta{
		Version: umoci.MetaVersion,
		From: casext.DescriptorPath{
			Walk: []ispec.Descriptor{dps[0].Descriptor()},
		},
	})

	if err != nil {
		return err
	}
	err = os.Chmod(path.Join(bundlePath, "rootfs"), 0755)
	return err
}
