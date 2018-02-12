package main

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	"github.com/urfave/cli"
)

var unladeCmd = cli.Command{
	Name:    "unlade",
	Usage:   "unpacks an OCI image to a directory",
	Aliases: []string{"unpack"},
	Action:  usernsWrapper(doUnlade),
	Flags:   []cli.Flag{},
}

func doUnlade(ctx *cli.Context) error {
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
		cmd := exec.Command(
			"umoci",
			"unpack",
			"--image",
			fmt.Sprintf("%s:%s", config.OCIDir, tag),
			path.Join(config.RootFSDir, tag))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("umoci unpack: %s: %s", err, string(output))
		}
		fmt.Printf(" - done.\n")
	}

	return nil
}
