package overlay

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker/container"
	"github.com/anuvu/stacker/lib"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/squashfs"
	"github.com/anuvu/stacker/storage"
	"github.com/anuvu/stacker/types"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func safeOverlayName(d digest.Digest) string {
	// dirs used in overlay lowerdir args can't have : in them, so lets
	// sanitize it
	return strings.ReplaceAll(d.String(), ":", "_")
}

func overlayPath(config types.StackerConfig, d digest.Digest, subdirs ...string) string {
	safeName := safeOverlayName(d)
	dirs := []string{config.RootFSDir, safeName}
	dirs = append(dirs, subdirs...)
	return path.Join(dirs...)
}

func (o *overlay) Unpack(tag, name string) error {
	cacheDir := path.Join(o.config.StackerDir, "layer-bases", "oci")
	oci, err := umoci.OpenLayout(cacheDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return err
	}

	pool := NewThreadPool(runtime.NumCPU())

	for _, layer := range manifest.Layers {
		contents := overlayPath(o.config, layer.Digest, "overlay")
		switch layer.MediaType {
		case stackeroci.MediaTypeLayerSquashfs:
			// don't really need to do this in parallel, but what
			// the hell.
			pool.Add(func(ctx context.Context) error {
				return container.RunUmociSubcommand(o.config, []string{
					"--tag", tag,
					"--oci-path", cacheDir,
					"--bundle-path", contents,
					"unpack-one",
					"--digest", layer.Digest.String(),
					"--squashfs",
				})
			})
		case ispec.MediaTypeImageLayer:
			fallthrough
		case ispec.MediaTypeImageLayerGzip:
			// don't extract things that have already been
			// extracted
			if _, err := os.Stat(contents); err == nil {
				continue
			}

			// TODO: when the umoci API grows support for uid
			// shifting, we can use the fancier features of context
			// cancelling in the thread pool...
			pool.Add(func(ctx context.Context) error {
				return container.RunUmociSubcommand(o.config, []string{
					"--tag", tag,
					"--oci-path", cacheDir,
					"--bundle-path", contents,
					"unpack-one",
					"--digest", layer.Digest.String(),
				})
			})
		default:
			return errors.Errorf("unknown media type %s", layer.MediaType)
		}
	}

	pool.DoneAddingJobs()

	err = pool.Run()
	if err != nil {
		return err
	}

	err = o.Create(name)
	if err != nil {
		return err
	}

	ovl, err := newOverlayMetadataFromOCI(oci, tag)
	if err != nil {
		return err
	}

	err = ovl.write(o.config, name)
	if err != nil {
		return err
	}

	return ovl.mount(o.config, name)
}

func (o *overlay) convertAndOutput(tag, name string, layerType types.LayerType) error {
	cacheDir := path.Join(o.config.StackerDir, "layer-bases", "oci")
	cacheOCI, err := umoci.OpenLayout(cacheDir)
	if err != nil {
		return err
	}
	defer cacheOCI.Close()

	oci, err := umoci.OpenLayout(o.config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	manifest, err := stackeroci.LookupManifest(cacheOCI, tag)
	if err != nil {
		return err
	}

	config, err := stackeroci.LookupConfig(cacheOCI, manifest.Config)
	if err != nil {
		return err
	}

	newManifest := manifest
	newManifest.Layers = []ispec.Descriptor{}

	newConfig := config
	newConfig.RootFS.DiffIDs = []digest.Digest{}

	for _, theLayer := range manifest.Layers {
		bundlePath := overlayPath(o.config, theLayer.Digest)
		overlayDir := path.Join(bundlePath, "overlay")

		var blob io.ReadCloser
		if layerType == "squashfs" {
			// sourced a non-squashfs image and wants a squashfs layer,
			// let's generate one.
			blob, err = squashfs.MakeSquashfs(o.config.OCIDir, overlayDir, nil)
			if err != nil {
				return err
			}
			defer blob.Close()
		} else {
			blob = layer.GenerateInsertLayer(overlayDir, "/", false, nil)
			defer blob.Close()
		}

		layerDigest, layerSize, err := oci.PutBlob(context.Background(), blob)
		if err != nil {
			return err
		}

		// slight hack, but this is much faster than a cp, and the
		// layers are the same, just in different formats
		err = os.Symlink(overlayPath(o.config, theLayer.Digest), overlayPath(o.config, layerDigest))
		if err != nil {
			return errors.Wrapf(err, "failed to create squashfs symlink")
		}

		layerMediaType := stackeroci.MediaTypeLayerSquashfs
		if layerType == "tar" {
			layerMediaType = ispec.MediaTypeImageLayerGzip
		}

		desc := ispec.Descriptor{
			MediaType: layerMediaType,
			Digest:    layerDigest,
			Size:      layerSize,
		}

		newManifest.Layers = append(newManifest.Layers, desc)
		newConfig.RootFS.DiffIDs = append(newConfig.RootFS.DiffIDs, layerDigest)
	}

	configDigest, configSize, err := oci.PutBlobJSON(context.Background(), newConfig)
	if err != nil {
		return err
	}

	newManifest.Config = ispec.Descriptor{
		MediaType: ispec.MediaTypeImageConfig,
		Digest:    configDigest,
		Size:      configSize,
	}

	manifestDigest, manifestSize, err := oci.PutBlobJSON(context.Background(), newManifest)
	if err != nil {
		return err
	}

	desc := ispec.Descriptor{
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}

	return oci.UpdateReference(context.Background(), layerType.LayerName(name), desc)
}

func lookupManifestInDir(dir, name string) (ispec.Manifest, error) {
	oci, err := umoci.OpenLayout(dir)
	if err != nil {
		return ispec.Manifest{}, err
	}
	defer oci.Close()

	return stackeroci.LookupManifest(oci, name)
}

func (o *overlay) initializeBasesInOutput(name string, layerTypes []types.LayerType, sfm types.StackerFiles) error {
	baseTag, baseLayer, err := storage.FindFirstBaseInOutput(name, sfm)
	if err != nil {
		return err
	}

	initialized := false
	if baseLayer != nil {
		if !baseLayer.BuildOnly && baseTag != name {
			// otherwise if it's already been built and the base
			// types match, import it from there
			for _, layerType := range layerTypes {
				err = lib.ImageCopy(lib.ImageCopyOpts{
					Src:  fmt.Sprintf("oci:%s:%s", o.config.OCIDir, layerType.LayerName(baseTag)),
					Dest: fmt.Sprintf("oci:%s:%s", o.config.OCIDir, layerType.LayerName(name)),
				})
				if err != nil {
					return err
				}
			}

			initialized = true
		} else if types.IsContainersImageLayer(baseLayer.From.Type) {
			cacheDir := path.Join(o.config.StackerDir, "layer-bases", "oci")
			cacheTag, err := baseLayer.From.ParseTag()
			if err != nil {
				return err
			}

			manifest, err := lookupManifestInDir(cacheDir, cacheTag)
			if err != nil {
				return err
			}

			sourceLayerType, err := types.NewLayerTypeManifest(manifest)
			if err != nil {
				return err
			}

			for _, layerType := range layerTypes {
				if sourceLayerType == layerType {
					err = lib.ImageCopy(lib.ImageCopyOpts{
						Src:  fmt.Sprintf("oci:%s:%s", cacheDir, cacheTag),
						Dest: fmt.Sprintf("oci:%s:%s", o.config.OCIDir, layerType.LayerName(name)),
					})
					if err != nil {
						return err
					}
				} else {
					err = o.convertAndOutput(cacheTag, name, layerType)
					if err != nil {
						return err
					}
				}
			}

			initialized = true
		}
	}

	if !initialized {
		oci, err := umoci.OpenLayout(o.config.OCIDir)
		if err != nil {
			return err
		}
		defer oci.Close()

		for _, layerType := range layerTypes {
			err = umoci.NewImage(oci, layerType.LayerName(name))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *overlay) Repack(name string, layerTypes []types.LayerType, sfm types.StackerFiles) error {
	err := o.initializeBasesInOutput(name, layerTypes, sfm)
	if err != nil {
		return err
	}

	// this is really just a wrapper for the function below, RepackOverlay.
	// we just do this dance so it's run in a userns and the uids look
	// right.
	args := []string{
		"--tag", name,
		"--oci-path", o.config.OCIDir,
		"repack-overlay",
	}

	for _, layerType := range layerTypes {
		args = append(args, "--layer-type", string(layerType))
	}
	return container.RunUmociSubcommand(o.config, args)
}

func generateLayer(config types.StackerConfig, mutators []*mutate.Mutator, name string, layerTypes []types.LayerType) (bool, error) {
	dir := path.Join(config.RootFSDir, name, "overlay")
	ents, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, errors.Wrapf(err, "coudln't read overlay path %s", dir)
	}

	if len(ents) == 0 {
		return false, nil
	}

	now := time.Now()
	history := &ispec.History{
		Created:    &now,
		CreatedBy:  fmt.Sprintf("stacker build of %s", name),
		EmptyLayer: false,
	}

	descs := []ispec.Descriptor{}
	for i, layerType := range layerTypes {
		mutator := mutators[i]
		var desc ispec.Descriptor
		if layerType == "tar" {
			// a hack, but GenerateInsertLayer() is the only thing that just takes
			// everything in a dir and makes it a layer.
			packOptions := layer.PackOptions{TranslateOverlayWhiteouts: true}
			uncompressed := layer.GenerateInsertLayer(dir, "/", false, &packOptions)
			defer uncompressed.Close()

			desc, err = mutator.Add(context.Background(), uncompressed, history, mutate.GzipCompressor)
			if err != nil {
				return false, err
			}
		} else {
			compressor := mutate.NewNoopCompressor(stackeroci.MediaTypeLayerSquashfs)
			blob, err := squashfs.MakeSquashfs(config.OCIDir, dir, nil)
			if err != nil {
				return false, err
			}
			defer blob.Close()

			desc, err = mutator.Add(context.Background(), blob, history, compressor)
			if err != nil {
				return false, err
			}
		}

		descs = append(descs, desc)
	}

	// we're going to update the manifest at the end with these generated
	// layers, so we need to "extract" them. but there's no need to
	// actually extract them, we can just rename the contents we already
	// have for the generated hash, since that's where it came from.
	target := overlayPath(config, descs[0].Digest)
	err = os.MkdirAll(target, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't make new layer overlay dir")
	}

	err = os.Rename(dir, path.Join(target, "overlay"))
	if err != nil {
		return false, errors.Wrapf(err, "couldn't move overlay data to new location")
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't re-make overlay contents for %s", dir)
	}

	// now, as we do in convertAndOutput, we make a symlink for the hash
	// for each additional layer type so that they see the same data
	for _, desc := range descs[1:] {
		err = os.Symlink(target, overlayPath(config, desc.Digest))
		if err != nil {
			return false, errors.Wrapf(err, "couldn't symlink additional layer type")
		}
	}

	return true, nil
}

func RepackOverlay(config types.StackerConfig, name string, layerTypes []types.LayerType) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	mutators := []*mutate.Mutator{}

	for _, layerType := range layerTypes {
		descPaths, err := oci.ResolveReference(context.Background(), layerType.LayerName(name))
		if err != nil {
			return err
		}

		mutator, err := mutate.New(oci, descPaths[0])
		if err != nil {
			return err
		}

		mutators = append(mutators, mutator)
	}

	ovl, err := readOverlayMetadata(config, name)
	if err != nil {
		return err
	}

	mutated := false
	// generate blobs for each build layer
	for _, buildLayer := range ovl.BuiltLayers {
		didMutate, err := generateLayer(config, mutators, buildLayer, layerTypes)
		if err != nil {
			return err
		}
		if didMutate {
			mutated = true
			ovl, err := readOverlayMetadata(config, buildLayer)
			if err != nil {
				return err
			}

			for i, layerType := range layerTypes {
				ovl.Manifests[layerType], err = mutators[i].Manifest(context.Background())
				if err != nil {
					return err
				}
			}

			err = ovl.write(config, buildLayer)
			if err != nil {
				return err
			}
		}
	}

	didMutate, err := generateLayer(config, mutators, name, layerTypes)
	if err != nil {
		return err
	}

	// if we didn't do anything, don't do anything :)
	if !didMutate && !mutated {
		return nil
	}

	// now, reset the overlay metadata; we can use the newly generated
	// manifest since we generated all the layers above.
	ovl = newOverlayMetadata()
	for i, layerType := range layerTypes {
		mutator := mutators[i]
		newPath, err := mutator.Commit(context.Background())
		if err != nil {
			return err
		}

		ovl.Manifests[layerType], err = mutator.Manifest(context.Background())
		if err != nil {
			return err
		}

		err = oci.UpdateReference(context.Background(), layerType.LayerName(name), newPath.Root())
		if err != nil {
			return err
		}
	}

	// unmount the old overlay
	err = unix.Unmount(path.Join(config.RootFSDir, name, "rootfs"), 0)
	if err != nil {
		return errors.Wrapf(err, "couldn't unmount old overlay")
	}

	err = ovl.write(config, name)
	if err != nil {
		return err
	}

	return ovl.mount(config, name)

}
