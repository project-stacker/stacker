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

	// TODO: this should be a lot better, we should use btrfs to do
	// manifest-by-manifest extracting. But that's more work, so let's do
	// this for now.
	for _, tag := range tags {
		err = s.Create(tag)
		if err != nil {
			return err
		}

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
	}

	return nil
}
