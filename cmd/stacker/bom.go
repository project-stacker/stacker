package main

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker-bom/pkg/bom"
	"stackerbuild.io/stacker-bom/pkg/distro"
	"stackerbuild.io/stacker-bom/pkg/fs"
)

var bomCmd = cli.Command{
	Name:  "bom",
	Usage: "work with a software bill of materials (BOM)",
	Subcommands: []*cli.Command{
		&cli.Command{
			Name:   "discover",
			Action: doBomDiscover,
		},
		&cli.Command{
			Name:   "build",
			Action: doBomBuild,
		},
		&cli.Command{
			Name:   "verify",
			Action: doBomVerify,
		},
	},
}

func doBomDiscover(ctx *cli.Context) error {
	author := "stacker-internal"
	org := "stacker-internal"

	if err := fs.Discover(author, org, "/stacker/artifacts/installed-packages.json"); err != nil {
		return nil
	}

	return nil
}

func doBomGenerate(ctx *cli.Context) error { //nolint:unused // used when invoked inside "run:"
	if ctx.Args().Len() != 1 {
		return errors.Errorf("wrong number of args for umount")
	}

	input := ctx.Args().Get(0)

	author := "stacker-internal"
	org := "stacker-internal"
	lic := "unknown"

	if err := distro.ParsePackage(input, author, org, lic, fmt.Sprintf("/stacker/artifacts/%s.json", filepath.Base(input))); err != nil {
		return nil
	}

	return nil
}

// build/roll your own sbom document for a particular dest (file/dir)
// by specifying details such as author, org, license, etc.
func doBomBuild(ctx *cli.Context) error {
	if ctx.Args().Len() < 7 {
		return errors.Errorf("wrong number of args")
	}

	dest := ctx.Args().Get(0)
	author := ctx.Args().Get(1)
	org := ctx.Args().Get(2)
	license := ctx.Args().Get(3)
	pkgname := ctx.Args().Get(4)
	pkgversion := ctx.Args().Get(5)
	paths := []string{}
	for i := 6; i < ctx.Args().Len(); i++ {
		paths = append(paths, ctx.Args().Get(i))
	}
	out := path.Join(dest, fmt.Sprintf("doc-%s.spdx.json", pkgname))
	name := fmt.Sprintf("doc-%s", pkgname)

	return fs.BuildPackage(name, author, org, license, pkgname, pkgversion, paths, out)
}

func doBomVerify(ctx *cli.Context) error {
	if ctx.Args().Len() != 4 {
		return errors.Errorf("wrong number of args")
	}

	dest := ctx.Args().Get(0)
	name := ctx.Args().Get(1)
	author := ctx.Args().Get(2)
	org := ctx.Args().Get(3)

	// first merge all individual sbom artifacts that may have been generated
	if err := bom.MergeDocuments("/stacker/artifacts", name, author, org, dest); err != nil {
		return err
	}

	// check against inventory
	if err := fs.GenerateInventory("/",
		[]string{"/proc", "/sys", "/dev", "/etc/resolv.conf", "/stacker"},
		"/stacker/artifacts/inventory.json"); err != nil {
		return err
	}

	return fs.Verify(dest, "/stacker/artifacts/inventory.json", "")
}
