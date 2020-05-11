package main

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/anuvu/stacker"
	"github.com/anuvu/stacker/lib"
	"github.com/anuvu/stacker/log"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/mutate"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	"github.com/openSUSE/umoci/pkg/fseval"
	"github.com/openSUSE/umoci/pkg/mtreefilter"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var umociCmd = cli.Command{
	Name:   "umoci",
	Hidden: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "tag",
			Usage: "tag in the oci image to operate on",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "name of the rootfs in --roots-dir",
		},
	},
	Subcommands: []cli.Command{
		cli.Command{
			Name:   "init",
			Action: doInit,
		},
		cli.Command{
			Name:   "unpack",
			Action: doUnpack,
		},
		cli.Command{
			Name:   "repack",
			Action: doRepack,
		},
	},
}

func doInit(ctx *cli.Context) error {
	name := ctx.GlobalString("tag")
	ociDir := config.OCIDir
	bundlePath := path.Join(ctx.GlobalString("roots-dir"), ctx.GlobalString("name"))
	var oci casext.Engine
	var err error

	if _, statErr := os.Stat(ociDir); statErr != nil {
		oci, err = umoci.CreateLayout(ociDir)
	} else {
		oci, err = umoci.OpenLayout(ociDir)
	}
	if err != nil {
		return errors.Wrapf(err, "Failed creating layout for %s", ociDir)
	}
	err = umoci.NewImage(oci, name)
	if err != nil {
		return errors.Wrapf(err, "umoci tag creation failed")
	}

	opts := layer.MapOptions{KeepDirlinks: true}
	err = umoci.Unpack(oci, name, bundlePath, opts, nil, ispec.Descriptor{})
	if err != nil {
		return errors.Wrapf(err, "umoci unpack failed for %s into %s", name, bundlePath)
	}

	return nil
}

func prepareUmociMetadata(storage stacker.Storage, name string, bundlePath string, dp casext.DescriptorPath, highestHash string) error {
	// We need the mtree metadata to be present, but since these
	// intermediate snapshots were created after each layer was
	// extracted and the metadata wasn't, it won't necessarily
	// exist. We could create it at extract time, but that would
	// make everything really slow, since we'd have to walk the
	// whole FS after every layer which would probably slow things
	// way down.
	//
	// Instead, check to see if the metadata has been generated. If
	// it hasn't, we generate it, and then re-snapshot back (since
	// we can't write to the old snapshots) with the metadata.
	//
	// This means the first restore will be slower, but after that
	// it will be very fast.
	//
	// A further complication is that umoci metadata is stored in terms of
	// the manifest that corresponds to the layers. When a config changes
	// (or e.g. a manifest is updated to reflect new layers), the old
	// manifest will be unreferenced and eventually GC'd. However, the
	// underlying layers were the same, since the hash here is the
	// aggregate hash of only the bits in the layers, and not of anything
	// related to the manifest. Then, when some "older" build comes along
	// referencing these same layers but with a different manifest, we'll
	// fail.
	//
	// Since the manifest doesn't actually affect the bits on disk, we can
	// essentially just copy the old manifest over to whatever the new
	// manifest will be if the hashes don't match. We re-snapshot since
	// snapshotting is generally cheap and we assume that the "new"
	// manifest will be the default. However, this code will still be
	// triggered if we go back to the old manifest.
	mtreeName := strings.Replace(dp.Descriptor().Digest.String(), ":", "_", 1)
	_, err := os.Stat(path.Join(bundlePath, "umoci.json"))
	if err == nil {
		mtreePath := path.Join(bundlePath, mtreeName+".mtree")
		_, err := os.Stat(mtreePath)
		if err == nil {
			// The best case: this layer's mtree and metadata match
			// what we're currently trying to extract. Do nothing.
			return nil
		}

		// The mtree file didn't match. Find the other mtree (it must
		// exist) in this directory (since any are necessarily correct
		// per above) and move it to this mtree name, then regenerate
		// umoci's metadata.
		entries, err := ioutil.ReadDir(bundlePath)
		if err != nil {
			return err
		}

		generated := false
		for _, ent := range entries {
			if !strings.HasSuffix(ent.Name(), ".mtree") {
				continue
			}

			generated = true
			oldMtreePath := path.Join(bundlePath, ent.Name())
			err = lib.FileCopy(mtreePath, oldMtreePath)
			if err != nil {
				return err
			}

			os.RemoveAll(oldMtreePath)
			break
		}

		if !generated {
			return errors.Errorf("couldn't find old umoci metadata in %s", bundlePath)
		}
	} else {
		// Umoci's metadata wasn't present. Let's generate it.
		log.Infof("generating mtree metadata for snapshot (this may take a bit)...")
		err = umoci.GenerateBundleManifest(mtreeName, bundlePath, fseval.DefaultFsEval)
		if err != nil {
			return err
		}
	}

	meta := umoci.Meta{
		Version:    umoci.MetaVersion,
		MapOptions: layer.MapOptions{},
		From:       dp,
	}

	err = umoci.WriteBundleMeta(bundlePath, meta)
	if err != nil {
		return err
	}

	err = storage.Delete(highestHash)
	if err != nil {
		return err
	}

	err = storage.Snapshot(name, highestHash)
	if err != nil {
		return err
	}

	return nil
}

// clean all the umoci metadata (config.json for the OCI runtime, umoci.json
// for its metadata, anything named *.mtree)
func cleanUmociMetadata(bundlePath string) error {
	ents, err := ioutil.ReadDir(bundlePath)
	if err != nil {
		return err
	}

	for _, ent := range ents {
		if ent.Name() == "rootfs" {
			continue
		}

		os.Remove(path.Join(bundlePath, ent.Name()))
	}

	return nil
}

func doUnpack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}

	storage, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}

	tag := ctx.GlobalString("tag")
	name := ctx.GlobalString("name")

	bundlePath := path.Join(ctx.GlobalString("roots-dir"), ctx.GlobalString("name"))

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return err
	}

	lastLayer := -1
	highestHash := ""
	for i, layerDesc := range manifest.Layers {
		hash, err := stacker.ComputeAggregateHash(manifest, layerDesc)
		if err != nil {
			return err
		}

		if storage.Exists(hash) {
			highestHash = hash
			lastLayer = i
			log.Infof("found previous extraction of %s", layerDesc.Digest.String())
		} else {
			break
		}
	}

	dps, err := oci.ResolveReference(context.Background(), tag)
	if err != nil {
		return err
	}

	if highestHash != "" {
		// Delete the previously created working snapshot; we're about
		// to create a new one.
		err = storage.Delete(name)
		if err != nil {
			return err
		}

		err = storage.Restore(highestHash, name)
		if err != nil {
			return err
		}

		// If we resotred from the last extracted layer, we can just
		// ensure the metadata is correct and return.
		if lastLayer+1 == len(manifest.Layers) {
			err = prepareUmociMetadata(storage, name, bundlePath, dps[0], highestHash)
			if err != nil {
				return err
			}

			return nil
		}
	}

	startFrom := manifest.Layers[lastLayer+1]

	// TODO: we could always share the empty layer, but that's more code
	// and seems extreme...
	callback := func(manifest ispec.Manifest, desc ispec.Descriptor) error {
		hash, err := stacker.ComputeAggregateHash(manifest, desc)
		if err != nil {
			return err
		}

		return storage.Snapshot(name, hash)
	}

	opts := layer.MapOptions{KeepDirlinks: true}
	// again, if we restored from something that already been unpacked but
	// we're going to unpack stuff on top of it, we need to delete the old
	// metadata.
	err = cleanUmociMetadata(bundlePath)
	if err != nil {
		return err
	}

	err = umoci.Unpack(oci, tag, bundlePath, opts, callback, startFrom)
	if err != nil {
		return err
	}

	// Ok, now that we have extracted and computed the mtree, let's
	// re-snapshot. The problem is that the snapshot in the callback won't
	// contain an mtree file, because the final mtree is generated after
	// the callback is called.
	hash, err := stacker.ComputeAggregateHash(manifest, manifest.Layers[len(manifest.Layers)-1])
	if err != nil {
		return err
	}
	err = storage.Delete(hash)
	if err != nil {
		return err
	}

	err = storage.Snapshot(name, hash)
	if err != nil {
		return err
	}

	return nil
}

func doRepack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}

	bundlePath := path.Join(ctx.GlobalString("roots-dir"), ctx.GlobalString("name"))
	meta, err := umoci.ReadBundleMeta(bundlePath)
	if err != nil {
		return err
	}

	mutator, err := mutate.New(oci, meta.From)
	if err != nil {
		return err
	}

	imageMeta, err := mutator.Meta(context.Background())
	if err != nil {
		return err
	}

	now := time.Now()
	history := &ispec.History{
		Author:     imageMeta.Author,
		Created:    &now,
		CreatedBy:  "stacker umoci repack",
		EmptyLayer: false,
	}

	filters := []mtreefilter.FilterFunc{stacker.LayerGenerationIgnoreRoot}
	return umoci.Repack(oci, ctx.GlobalString("tag"), bundlePath, meta, history, filters, true, mutator)
}
