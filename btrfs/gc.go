package btrfs

import (
	"context"
	"io/ioutil"
	"path"

	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/opencontainers/umoci"
)

func gcForOCILayout(s *btrfs, layout string, thingsToKeep map[string]bool) error {
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
			hash, err := ComputeAggregateHash(manifest, layer)
			if err != nil {
				return err
			}

			thingsToKeep[hash] = true
		}
	}

	return nil
}

func (b *btrfs) GC() error {
	thingsToKeep := map[string]bool{}

	err := gcForOCILayout(b, b.c.OCIDir, thingsToKeep)
	if err != nil {
		return err
	}

	err = gcForOCILayout(b, path.Join(b.c.StackerDir, "layer-bases", "oci"), thingsToKeep)
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(b.c.RootFSDir)
	if err != nil {
		return err
	}

	for _, ent := range entries {
		_, used := thingsToKeep[ent.Name()]
		if used {
			continue
		}

		err = b.Delete(ent.Name())
		if err != nil {
			return err
		}
	}

	return nil
}
