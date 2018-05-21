package main

import (
	"encoding/json"
	"fmt"

	"github.com/openSUSE/umoci"
	"github.com/urfave/cli"
)

var inspectCmd = cli.Command{
	Name:   "inspect",
	Usage:  "print the json representation of an OCI image",
	Action: doInspect,
	Flags:  []cli.Flag{},
	ArgsUsage: `[tag]

<tag> is the tag in the stackerfile to inspect. If none is supplied, inspect
prints the information on all tags.`,
}

func doInspect(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}

	arg := ctx.Args().Get(0)
	if arg != "" {
		return renderManifest(oci, arg)
	}

	tags, err := oci.ListTags()
	if err != nil {
		return err
	}

	for _, t := range tags {
		err = renderManifest(oci, t)
		if err != nil {
			return err
		}
	}

	return nil
}

func renderManifest(oci *umoci.Layout, name string) error {
	man, err := oci.LookupManifest(name)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", name)
	for i, l := range man.Layers {
		fmt.Printf("\tlayer %d: %s\n", i, l.Digest)
	}

	if len(man.Annotations) > 0 {
		fmt.Printf("Annotations:\n")
		for k, v := range man.Annotations {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	config, err := oci.LookupConfig(man.Config)
	if err != nil {
		return err
	}

	fmt.Printf("Image config:\n")
	pretty, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}
