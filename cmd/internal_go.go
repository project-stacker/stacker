package main

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/anuvu/stacker/btrfs"
	"github.com/anuvu/stacker/lib"
	"github.com/anuvu/stacker/log"
	stackermtree "github.com/anuvu/stacker/mtree"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/overlay"
	"github.com/anuvu/stacker/squashfs"
	"github.com/anuvu/stacker/types"
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
	"golang.org/x/sys/unix"
)

var internalGoCmd = cli.Command{
	Name:   "internal-go",
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
			Name:   "init-empty",
			Action: doInitEmpty,
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
				cli.BoolFlag{
					Name:  "squashfs",
					Usage: "unpack as squashfs",
				},
			},
		},
		cli.Command{
			Name:   "repack-overlay",
			Action: doRepackOverlay,
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:  "layer-type",
					Usage: "set the output layer type (supported values: tar, squashfs); can be supplied multiple times",
					Value: &cli.StringSlice{"tar"},
				},
			},
		},
		cli.Command{
			Name:   "generate-bundle-manifest",
			Action: doGenerateBundleManifest,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "mtree-name",
					Usage: "the name of the file to write mtree data to",
				},
			},
		},
		cli.Command{
			Name:   "check-overlay",
			Action: doCheckOverlay,
		},
		cli.Command{
			Name:   "unpack-tar",
			Action: doUnpackTar,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "tar",
					Usage: "the name of the tar file to extract",
				},
				cli.StringFlag{
					Name:  "dest-dir",
					Usage: "the name of the tar file to extract",
				},
			},
		},
	},
	Before: doBeforeUmociSubcommand,
}

func doBeforeUmociSubcommand(ctx *cli.Context) error {
	log.Debugf("stacker subcommand: %v", os.Args)
	return nil
}

func doCheckOverlay(ctx *cli.Context) error {
	return overlay.CanDoOverlay(config)
}

func doInitEmpty(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-path")
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

	err = umoci.NewImage(oci, tag)
	if err != nil {
		return err
	}

	// kind of a hack, but the API won't let us init an empty image in a
	// bundle with data already in it, which is probably reasonable. so
	// what we do instead is: unpack the empty image above into a temp
	// directory, then copy the mtree/umoci metadata over to our rootfs.
	dir, err := ioutil.TempDir("", "umoci-init-empty")
	if err != nil {
		return errors.Wrapf(err, "couldn't create temp dir")
	}
	defer os.RemoveAll(dir)

	err = doDoUnpack(tag, ociDir, dir, "")
	if err != nil {
		return err
	}

	ents, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "couldn't read temp dir")
	}

	for _, ent := range ents {
		if ent.Name() == "rootfs" {
			continue
		}

		// copy all metadata to the real dir
		err = lib.FileCopy(path.Join(bundlePath, ent.Name()), path.Join(dir, ent.Name()))
		if err != nil {
			return err
		}
	}

	return nil
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

func which(name string) string {
	return whichSearch(name, strings.Split(os.Getenv("PATH"), ":"))
}

func whichSearch(name string, paths []string) string {
	var search []string

	if strings.ContainsRune(name, os.PathSeparator) {
		if path.IsAbs(name) {
			search = []string{name}
		} else {
			search = []string{"./" + name}
		}
	} else {
		search = []string{}
		for _, p := range paths {
			search = append(search, path.Join(p, name))
		}
	}

	for _, fPath := range search {
		if err := unix.Access(fPath, unix.X_OK); err == nil {
			return fPath
		}
	}

	return ""
}

func extractSingleSquash(squashFile string, extractDir string) error {
	err := os.MkdirAll(extractDir, 0755)
	if err != nil {
		return err
	}

	var uCmd []string
	if config.StorageType == "btrfs" {
		if which("squashtool") == "" {
			return errors.Errorf("must have squashtool (https://github.com/anuvu/squashfs) to correctly extract squashfs using btrfs storage backend")
		}

		uCmd = []string{"squashtool", "extract", "--whiteouts", "--perms",
			"--devs", "--sockets", "--owners"}
		uCmd = append(uCmd, squashFile, extractDir)
	} else {
		uCmd = []string{"unsquashfs", "-f", "-d", extractDir, squashFile}
	}

	cmd := exec.Command(uCmd[0], uCmd[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
		err = extractSingleSquash(squashfsFile, rootfs)
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

// heh, dodo
func doDoUnpack(tag, ociDir, bundlePath, startFromDigest string) error {
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
		if desc.Digest.String() == startFromDigest {
			startFrom = desc
			break
		}
	}

	if startFromDigest != "" && startFrom.MediaType == "" {
		return errors.Errorf("couldn't find starting hash %s", startFromDigest)
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

func doUnpack(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-path")
	bundlePath := ctx.GlobalString("bundle-path")
	startFrom := ctx.String("start-from")

	return doDoUnpack(tag, ociDir, bundlePath, startFrom)
}

func doRepack(ctx *cli.Context) error {
	tag := ctx.GlobalString("tag")
	ociDir := ctx.GlobalString("oci-path")
	bundlePath := ctx.GlobalString("bundle-path")

	layerType := types.LayerType(ctx.String("layer-type"))

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

	layerName := layerType.LayerName(tag)
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
		return umoci.Repack(oci, layerName, bundlePath, meta, history, filters, true, mutator)
	case "squashfs":
		return squashfs.GenerateSquashfsLayer(layerName, imageMeta.Author, bundlePath, ociDir, oci)
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

	if ctx.Bool("squashfs") {
		return extractSingleSquash(
			path.Join(ociDir, "blobs", "sha256", digest.Encoded()),
			path.Join(bundlePath, "rootfs"))
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
	layerTypes, err := types.NewLayerTypes(ctx.StringSlice("layer-type"))
	if err != nil {
		return err
	}

	return overlay.RepackOverlay(config, tag, layerTypes)
}

func doGenerateBundleManifest(ctx *cli.Context) error {
	bundlePath := ctx.GlobalString("bundle-path")
	mtreeName := ctx.String("mtree-name")
	return umoci.GenerateBundleManifest(mtreeName, bundlePath, fseval.Default)
}

func doUnpackTar(ctx *cli.Context) error {
	destDir := ctx.String("dest-dir")
	tar := ctx.String("tar")
	tarReader, err := os.Open(tar)
	if err != nil {
		return errors.Wrapf(err, "couldn't open %s", tar)
	}
	defer tarReader.Close()
	var uncompressed io.ReadCloser
	uncompressed, err = pgzip.NewReader(tarReader)
	if err != nil {
		_, err = tarReader.Seek(0, os.SEEK_SET)
		if err != nil {
			return errors.Wrapf(err, "failed to 0 seek %s", tar)
		}
		uncompressed = tarReader
	} else {
		defer uncompressed.Close()
	}

	return layer.UnpackLayer(destDir, uncompressed, &layer.UnpackOptions{KeepDirlinks: true})
}
