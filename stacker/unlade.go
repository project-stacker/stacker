package main

import (
	"fmt"
	"os"
	"path"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	"github.com/urfave/cli"
)

var unladeCmd = cli.Command{
	Name:    "unlade",
	Usage:   "unpacks an OCI image to a directory",
	Aliases: []string{"unpack"},
	Action:  doUnlade,
	Flags:   []cli.Flag{},
}

func doUnlade(ctx *cli.Context) error {
	if _, err := os.Stat(config.OCIDir); err != nil {
		return err
	}

	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}

	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	tags, err := oci.ListTags()
	if err != nil {
		return err
	}

	fmt.Printf("Unpacking all layers from %s into %s\n", config.OCIDir, config.RootFSDir)
	// TODO: this should be a lot better, we should use btrfs to do
	// manifest-by-manifest extracting. But that's more work, so let's do
	// this for now.
	for idx, tag := range tags {
		err = s.Create(tag)
		if err != nil {
			return err
		}
		fmt.Printf("%d/%d: unpacking %s", idx+1, len(tags), tag)
		args := []string{
			"umoci",
			"unpack",
			"--image",
			fmt.Sprintf("%s:%s", config.OCIDir, tag),
			path.Join(config.RootFSDir, tag),
		}
		err = stacker.MaybeRunInUserns(args, "unpack failed")
		if err != nil {
			return err
		}
		fmt.Printf(" - done.\n")
	}

	return nil
}
