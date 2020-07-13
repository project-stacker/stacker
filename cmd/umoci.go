package main

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/anuvu/stacker/btrfs"
	"github.com/anuvu/stacker/log"
	stackermtree "github.com/anuvu/stacker/mtree"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/overlay"
	"github.com/anuvu/stacker/squashfs"
	"github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/mtreefilter"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var umociCmd = cli.Command{
	// TODO: rename this, it's not really "umoci" any more.
	Name:   "umoci",
	Hidden: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "tag",
			Usage: "tag in the oci image to operate on",
		},
		cli.StringFlag{
			Name:  "bundle-path",
			Usage: "the bundle path to operate on",
		},
		cli.StringFlag{
			Name:  "oci-path",
			Usage: "the OCI layout to operate on",
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
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "start-from",
					Usage: "hash to start from if any",
				},
			},
		},
		cli.Command{
			Name:   "repack",
			Action: doRepack,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "layer-type",
					Usage: "layer type to emit when repacking",
				},
			},
		},
		cli.Command{
			Name:   "unpack-one",
			Action: doUnpackOne,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "digest",
					Usage: "digest of the layer to unpack",
				},
			},
		},
		cli.Command{
			Name:   "repack-overlay",
			Action: doRepackOverlay,
		},
	},
	Before: doBeforeUmociSubcommand,
}

func doBeforeUmociSubcommand(ctx *cli.Context) error {
	log.Debugf("stacker subcommand: %v", os.Args)
	return nil
}

func doInit(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-path")
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
	return umoci.NewImage(oci, tag)
}

func tarUnpack(oci casext.Engine, tag string, bundlePath string, callback layer.AfterLayerUnpackCallback, startFrom ispec.Descriptor) error {
	whiteoutMode := layer.OCIStandardWhiteout
	if config.StorageType == "overlay" {
		whiteoutMode = layer.OverlayFSWhiteout
	}

	opts := layer.UnpackOptions{
		KeepDirlinks:     true,
		AfterLayerUnpack: callback,
		StartFrom:        startFrom,
		WhiteoutMode:     whiteoutMode,
	}
	return umoci.Unpack(oci, tag, bundlePath, opts)
}

func squashfsUnpack(ociDir string, oci casext.Engine, tag string, bundlePath string, callback layer.AfterLayerUnpackCallback, startFrom ispec.Descriptor) error {
	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return err
	}

	found := false
	for _, layer := range manifest.Layers {
		if !found && startFrom.MediaType != "" && layer.Digest.String() != startFrom.Digest.String() {
			continue
		}
		found = true

		rootfs := path.Join(bundlePath, "rootfs")
		squashfsFile := path.Join(ociDir, "blobs", "sha256", layer.Digest.Encoded())
		userCmd := []string{"unsquashfs", "-f", "-d", rootfs, squashfsFile}
		cmd := exec.Command(userCmd[0], userCmd[1:]...)
		cmd.Stdin = nil
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
		err = callback(manifest, layer)
		if err != nil {
			return err
		}
	}

	dps, err := oci.ResolveReference(context.Background(), tag)
	if err != nil {
		return err
	}

	mtreeName := strings.Replace(dps[0].Descriptor().Digest.String(), ":", "_", 1)
	err = umoci.GenerateBundleManifest(mtreeName, bundlePath, fseval.Rootless)
	if err != nil {
		return err
	}

	err = umoci.WriteBundleMeta(bundlePath, umoci.Meta{
		Version: umoci.MetaVersion,
		From: casext.DescriptorPath{
			Walk: []ispec.Descriptor{dps[0].Descriptor()},
		},
	})

	if err != nil {
		return err
	}
	return nil
}

func doUnpack(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-path")
	bundlePath := ctx.GlobalString("bundle-path")

	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	// Other unpack drivers will probably want to do something fancier for
	// their unpacks and will exec a different code path, so we can/should
	// assume this is btrfs for now. Additionally, we can assume its an
	// existing btrfs, since the loopback device should have been mounted
	// by the parent.
	storage := btrfs.NewExisting(config)
	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return err
	}

	startFrom := ispec.Descriptor{}
	for _, desc := range manifest.Layers {
		if desc.Digest.String() == ctx.String("start-from") {
			startFrom = desc
			break
		}
	}

	if ctx.String("start-from") != "" && startFrom.MediaType == "" {
		return errors.Errorf("couldn't find starting hash %s", ctx.String("start-from"))
	}

	var callback layer.AfterLayerUnpackCallback
	if config.StorageType == "btrfs" {
		// TODO: we could always share the empty layer, but that's more code
		// and seems extreme...
		callback = func(manifest ispec.Manifest, desc ispec.Descriptor) error {
			hash, err := btrfs.ComputeAggregateHash(manifest, desc)
			if err != nil {
				return err
			}

			log.Debugf("creating intermediate snapshot %s", hash)
			return storage.Snapshot(path.Base(bundlePath), hash)
		}
	}

	if len(manifest.Layers) != 0 && manifest.Layers[0].MediaType == stackeroci.MediaTypeLayerSquashfs {
		return squashfsUnpack(ociDir, oci, tag, bundlePath, callback, startFrom)
	}

	return tarUnpack(oci, tag, bundlePath, callback, startFrom)
}

func doRepack(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-path")
	bundlePath := ctx.GlobalString("bundle-path")

	layerType := ctx.String("layer-type")

	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return err
	}
	defer oci.Close()

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

	switch layerType {
	case "tar":
		now := time.Now()
		history := &ispec.History{
			Author:     imageMeta.Author,
			Created:    &now,
			CreatedBy:  "stacker umoci repack",
			EmptyLayer: false,
		}

		filters := []mtreefilter.FilterFunc{stackermtree.LayerGenerationIgnoreRoot}
		return umoci.Repack(oci, tag, bundlePath, meta, history, filters, true, mutator)
	case "squashfs":
		return squashfs.GenerateSquashfsLayer(tag, imageMeta.Author, bundlePath, ociDir, oci)
	default:
		return errors.Errorf("unknown layer type %s", layerType)
	}
}

func doUnpackOne(ctx *cli.Context) error {
	ociDir := ctx.GlobalString("oci-path")
	bundlePath := ctx.GlobalString("bundle-path")
	digest, err := digest.Parse(ctx.String("digest"))
	if err != nil {
		return err
	}

	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	compressed, err := oci.GetBlob(context.Background(), digest)
	if err != nil {
		return err
	}
	defer compressed.Close()

	uncompressed, err := pgzip.NewReader(compressed)
	if err != nil {
		return err
	}

	return layer.UnpackLayer(bundlePath, uncompressed, nil)
}

func doRepackOverlay(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	return overlay.RepackOverlay(config, tag)
}
