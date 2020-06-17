package main

import (
	"context"
	"io/ioutil"
	"path"

	"github.com/anuvu/stacker"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/opencontainers/umoci"
	"github.com/urfave/cli"
)

var gcCmd = cli.Command{
	Name:   "gc",
	Usage:  "gx unused OCI imports/outputs and btrfs snapshots",
	Action: doGC,
}

func gcForOCILayout(s stacker.Storage, layout string, thingsToKeep map[string]bool) error {
	oci, err := umoci.OpenLayout(layout)
	if err != nil {
		return err
	}
	defer oci.Close()

	err = oci.GC(context.Background())
	if err != nil {
		return err
	}

	tags, err := oci.ListReferences(context.Background())
	if err != nil {
		return err
	}

	for _, t := range tags {
		manifest, err := stackeroci.LookupManifest(oci, t)
		if err != nil {
			return err
		}

		// keep both tags and hashes
		thingsToKeep[t] = true

		for _, layer := range manifest.Layers {
			hash, err := stacker.ComputeAggregateHash(manifest, layer)
			if err != nil {
				return err
			}

			thingsToKeep[hash] = true
		}
	}

	return nil
}

func doGC(ctx *cli.Context) error {
	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer s.Detach()

	thingsToKeep := map[string]bool{}

	err = gcForOCILayout(s, config.OCIDir, thingsToKeep)
	if err != nil {
		return err
	}

	err = gcForOCILayout(s, path.Join(config.StackerDir, "layer-bases", "oci"), thingsToKeep)
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(config.RootFSDir)
	if err != nil {
		return err
	}

	for _, ent := range entries {
		_, used := thingsToKeep[ent.Name()]
		if used {
			continue
		}

		err = s.Delete(ent.Name())
		if err != nil {
			return err
		}
	}

	return err
}
