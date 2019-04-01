package main

import (
	"context"
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

	tags, err := oci.ListReferences(context.Background())
	if err != nil {
		return err
	}

	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}

	fmt.Printf("Unpacking all layers from %s into %s\n", config.OCIDir, config.RootFSDir)
	for idx, tag := range tags {
		err = s.Create(tag)
		if err != nil {
			return err
		}
		fmt.Printf("%d/%d: unpacking %s", idx+1, len(tags), tag)
		args := []string{
			binary,
			"--oci-dir", config.OCIDir,
			"umoci",
			"--tag", tag,
			"--bundle-path", path.Join(config.RootFSDir, tag),
			"unpack",
		}
		err = stacker.MaybeRunInUserns(args, "unpack failed")
		if err != nil {
			return err
		}
		fmt.Printf(" - done.\n")
	}

	return nil
}
