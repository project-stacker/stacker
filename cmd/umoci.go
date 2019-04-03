package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/anuvu/stacker"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/mutate"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	"github.com/openSUSE/umoci/pkg/fseval"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var umociCmd = cli.Command{
	Name:   "umoci",
	Hidden: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name: "bundle-path",
		},
		cli.StringFlag{
			Name: "tag",
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
			Flags: []cli.Flag{
				cli.Uint64Flag{
					Name: "max-layer-size",
				},
			},
		},
	},
}

func doInit(ctx *cli.Context) error {
	name := ctx.GlobalString("tag")
	ociDir := config.OCIDir
	bundlePath := ctx.GlobalString("bundle-path")
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
	bundlePath := ctx.GlobalString("bundle-path")

	manifest, err := stacker.LookupManifest(oci, tag)
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
			fmt.Println("found previous extraction of", layerDesc.Digest.String())
		} else {
			break
		}
	}

	dps, err := oci.ResolveReference(context.Background(), tag)
	if err != nil {
		return err
	}

	// generate the metadata
	mtreeName := strings.Replace(dps[0].Descriptor().Digest.String(), ":", "_", 1)

	if highestHash != "" {
		// Delete the previously created working snapshot; we're about
		// to create a new one.
		err = storage.Delete(".working")
		if err != nil {
			return err
		}

		// TODO: this is a little wonky: we're assuming that
		// bundle-path ends in .working. It always does because
		// this is an internal API, but we should refactor this
		// a bit.
		err = storage.Restore(highestHash, ".working")
		if err != nil {
			return err
		}

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
		_, err := os.Stat(path.Join(bundlePath, "umoci.json"))
		if err != nil {
			fmt.Println("generating mtree metadata for snapshot (this may take a bit)...")
			meta := umoci.Meta{
				Version:    umoci.MetaVersion,
				MapOptions: layer.MapOptions{},
				From:       dps[0],
			}

			// apply: may have generated an mtree file at some
			// point, and since GenerateBundleManifest() fails if
			// this file already exists, let's just try to remove
			// it. see a corresponding comment in apply.go
			os.RemoveAll(path.Join(bundlePath, mtreeName+".mtree"))
			err = umoci.GenerateBundleManifest(mtreeName, bundlePath, fseval.DefaultFsEval)
			if err != nil {
				return err
			}

			err = umoci.WriteBundleMeta(bundlePath, meta)
			if err != nil {
				return err
			}

			err = storage.Delete(highestHash)
			if err != nil {
				return err
			}

			err = storage.Snapshot(".working", highestHash)
			if err != nil {
				return err
			}
		}
	}

	// If we restored from the last extracted layer, we don't need to do
	// anything, and can just return.
	if lastLayer >= 0 && lastLayer+1 == len(manifest.Layers) {
		return nil
	}
	startFrom := manifest.Layers[lastLayer+1]

	// TODO: we could always share the empty layer, but that's more code
	// and seems extreme...
	callback := func(manifest ispec.Manifest, desc ispec.Descriptor) error {
		hash, err := stacker.ComputeAggregateHash(manifest, desc)
		if err != nil {
			return err
		}

		return storage.Snapshot(".working", hash)
	}

	opts := layer.MapOptions{KeepDirlinks: true}
	// again, if we restored from something that already had an mtree
	// entry, but are going to unpack stuff on top of it, umoci will fail.
	// So let's delete this, because umoci is going to create it again
	// anyways.
	os.RemoveAll(path.Join(bundlePath, mtreeName+".mtree"))
	return umoci.Unpack(oci, ctx.GlobalString("tag"), bundlePath, opts, callback, startFrom)
}

func doRepack(ctx *cli.Context) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}

	bundlePath := ctx.GlobalString("bundle-path")
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

	return umoci.Repack(oci, ctx.GlobalString("tag"), bundlePath, meta, history, nil, true, mutator)
}
