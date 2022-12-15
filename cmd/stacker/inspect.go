package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dustin/go-humanize"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	stackeroci "stackerbuild.io/stacker/pkg/oci"
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
	defer oci.Close()

	arg := ctx.Args().Get(0)
	if arg != "" {
		return renderManifest(oci, arg)
	}

	tags, err := oci.ListReferences(context.Background())
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

func renderManifest(oci casext.Engine, name string) error {
	man, err := stackeroci.LookupManifest(oci, name)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", name)
	for i, l := range man.Layers {
		fmt.Printf("\tlayer %d: %s... (%s, %s)\n", i, l.Digest.Encoded()[:12], humanize.Bytes(uint64(l.Size)), l.MediaType)
	}

	if len(man.Annotations) > 0 {
		fmt.Printf("Annotations:\n")
		for k, v := range man.Annotations {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	configBlob, err := oci.FromDescriptor(context.Background(), man.Config)
	if err != nil {
		return err
	}

	if configBlob.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return errors.Errorf("bad image config type: %s", configBlob.Descriptor.MediaType)
	}

	config := configBlob.Data.(ispec.Image)

	fmt.Printf("Image config:\n")
	pretty, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}
