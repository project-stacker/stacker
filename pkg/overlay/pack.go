package overlay

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"github.com/pkg/xattr"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/log"
	stackeroci "stackerbuild.io/stacker/pkg/oci"
	"stackerbuild.io/stacker/pkg/squashfs"
	"stackerbuild.io/stacker/pkg/storage"
	"stackerbuild.io/stacker/pkg/types"
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
		digest := layer.Digest
		contents := overlayPath(o.config, digest, "overlay")
		if squashfs.IsSquashfsMediaType(layer.MediaType) {
			// don't really need to do this in parallel, but what
			// the hell.
			pool.Add(func(ctx context.Context) error {
				return unpackOne(cacheDir, contents, digest, true)
			})
		} else {
			switch layer.MediaType {
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
					return unpackOne(cacheDir, contents, digest, false)
				})
			default:
				return errors.Errorf("unknown media type %s", layer.MediaType)
			}
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

	return ovl.write(o.config, name)
}

func ConvertAndOutput(config types.StackerConfig, tag, name string, layerType types.LayerType) error {
	cacheDir := path.Join(config.StackerDir, "layer-bases", "oci")
	cacheOCI, err := umoci.OpenLayout(cacheDir)
	if err != nil {
		return err
	}
	defer cacheOCI.Close()

	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	manifest, err := stackeroci.LookupManifest(cacheOCI, tag)
	if err != nil {
		return err
	}

	imageConfig, err := stackeroci.LookupConfig(cacheOCI, manifest.Config)
	if err != nil {
		return err
	}

	newManifest := manifest
	newManifest.Layers = []ispec.Descriptor{}

	newConfig := imageConfig
	newConfig.RootFS.DiffIDs = []digest.Digest{}

	for _, theLayer := range manifest.Layers {
		bundlePath := overlayPath(config, theLayer.Digest)
		overlayDir := path.Join(bundlePath, "overlay")
		// generate blob
		blob, mediaType, rootHash, err := generateBlob(layerType, overlayDir, config.OCIDir)
		if err != nil {
			return err
		}
		defer blob.Close()
		// add it to the oci repository
		desc, err := ociPutBlob(blob, config, mediaType, rootHash)
		if err != nil {
			return err
		}

		// slight hack, but this is much faster than a cp, and the
		// layers are the same, just in different formats
		err = os.Symlink(overlayPath(config, theLayer.Digest), overlayPath(config, desc.Digest))
		if err != nil {
			return errors.Wrapf(err, "failed to create squashfs symlink")
		}
		newManifest.Layers = append(newManifest.Layers, desc)
		newConfig.RootFS.DiffIDs = append(newConfig.RootFS.DiffIDs, desc.Digest)
	}
	// update image
	_, err = stackeroci.UpdateImageConfig(oci, layerType.LayerName(name), newConfig, newManifest)
	if err != nil {
		return err
	}

	return nil
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
	baseTag, baseLayer, foundBase, err := storage.FindFirstBaseInOutput(name, sfm)
	if err != nil {
		return err
	}

	initialized := false
	if foundBase {
		if !baseLayer.BuildOnly && baseTag != name {
			// otherwise if it's already been built and the base
			// types match, import it from there
			for _, layerType := range layerTypes {
				log.Debugf("Running image copy to oci:%s:%s", o.config.OCIDir, layerType.LayerName(name))
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
					log.Debugf("converting between %v and %v", sourceLayerType, layerType)
					err = ConvertAndOutput(o.config, cacheTag, name, layerType)
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

	return repackOverlay(o.config, name, layerTypes)
}

// generateBlob generates either a tar blob or a squashfs blob based on layerType
func generateBlob(layerType types.LayerType, contents string, ociDir string) (io.ReadCloser, string, string, error) {
	var blob io.ReadCloser
	var err error
	var mediaType string
	var rootHash string
	if layerType.Type == "tar" {
		packOptions := layer.RepackOptions{TranslateOverlayWhiteouts: true}
		blob = layer.GenerateInsertLayer(contents, "/", false, &packOptions)
		mediaType = ispec.MediaTypeImageLayer
	} else {
		blob, mediaType, rootHash, err = squashfs.MakeSquashfs(ociDir, contents, nil, layerType.Verity)
		if err != nil {
			return nil, "", "", err
		}
	}
	return blob, mediaType, rootHash, nil
}

// ociPutBlob takes a tar/squashfs blob and adds it into the oci repository
func ociPutBlob(blob io.ReadCloser, config types.StackerConfig, layerMediaType string, rootHash string) (ispec.Descriptor, error) {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return ispec.Descriptor{}, err
	}
	defer oci.Close()

	layerDigest, layerSize, err := oci.PutBlob(context.Background(), blob)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	annotations := map[string]string{}
	if rootHash != "" {
		annotations[squashfs.VerityRootHashAnnotation] = rootHash
	}

	desc := ispec.Descriptor{
		MediaType:   layerMediaType,
		Digest:      layerDigest,
		Size:        layerSize,
		Annotations: annotations,
	}

	return desc, nil
}

func stripOverlayAttrs(path string) error {
	attrs, err := xattr.LList(path)
	if err != nil {
		return err
	}
	const match = ".overlay."
	const opaque = match + "opaque"

	dropped := []string{}
	for _, attr := range attrs {
		if !strings.Contains(attr, match) {
			continue
		}
		if strings.HasSuffix(attr, opaque) {
			continue
		}
		if err := xattr.LRemove(path, attr); err != nil {
			return errors.Errorf("%s: failed to remove attr %s: %v", path, attr, err)
		}
		dropped = append(dropped, attr)
	}

	if len(dropped) != 0 {
		log.Debugf("%s: dropped overlay attrs: %s", path, strings.Join(dropped, ","))
	}
	return nil
}

func stripOverlayAttrsUnder(dirPath string) error {
	return fs.WalkDir(os.DirFS(dirPath), ".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			p := filepath.Join(dirPath, path)
			if lib.IsSymlink(p) {
				// user.* xattrs "can not" exist on symlinks
				return nil
			}
			return stripOverlayAttrs(p)
		})
}

func generateLayer(config types.StackerConfig, oci casext.Engine, mutators []*mutate.Mutator, name string, layerTypes []types.LayerType) (bool, error) {
	dir := path.Join(config.RootFSDir, name, "overlay")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false, errors.Wrapf(err, "coudln't read overlay path %s", dir)
	}

	if len(ents) == 0 {
		ovl, err := readOverlayMetadata(config, name)
		if err != nil {
			return false, err
		}
		if ovl.HasBuiltOCIOutput {
			for i, layerType := range layerTypes {
				manifest := ovl.Manifests[layerType]
				layer := manifest.Layers[len(manifest.Layers)-1]

				config := ovl.Configs[layerType]

				diffID := config.RootFS.DiffIDs[len(config.RootFS.DiffIDs)-1]
				history := config.History[len(config.History)-1]

				mutator := mutators[i]
				err = mutator.AddExisting(context.Background(), layer, &history, diffID)
				if err != nil {
					return false, err
				}
			}

			return true, nil
		}
		return false, nil
	}

	now := time.Now()
	history := &ispec.History{
		Created:    &now,
		CreatedBy:  fmt.Sprintf("stacker build of %s", name),
		EmptyLayer: false,
	}

	if err := stripOverlayAttrsUnder(dir); err != nil {
		return false, err
	}

	descs := []ispec.Descriptor{}
	for i, layerType := range layerTypes {
		mutator := mutators[i]
		var desc ispec.Descriptor

		blob, mediaType, rootHash, err := generateBlob(layerType, dir, config.OCIDir)
		if err != nil {
			return false, err
		}
		defer blob.Close()

		if layerType.Type == "tar" {
			desc, err = mutator.Add(context.Background(), mediaType, blob, history, mutate.GzipCompressor, nil)
			if err != nil {
				return false, err
			}
		} else {
			annotations := map[string]string{}
			if rootHash != "" {
				annotations[squashfs.VerityRootHashAnnotation] = rootHash
			}
			desc, err = mutator.Add(context.Background(), mediaType, blob, history, mutate.NoopCompressor, annotations)
			if err != nil {
				return false, err
			}
		}
		log.Debugf("generated %v layer %s from %s", layerType, desc.Digest, dir)

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
		if !os.IsExist(err) {
			return false, errors.Wrapf(err, "couldn't move overlay data to new location")
		}
		// however, it's possible that we've *already* generated a layer that
		// has this hash. This can happen when two filesystems are based on the
		// same type: built rfs but make no local changes. They will have
		// non-empty overlay/ dirs since the content will be from the previous
		// type: built layer things were based on, but it will be identical
		// since the builds made no additional changes. In this case, it's safe
		// to just delete this layer's overlay/ and symlink it into the right
		// place below, instead of moving it.
		log.Debugf("target exists, simply removing %s", dir)
		err = os.RemoveAll(dir)
		if err != nil {
			return false, errors.Wrapf(err, "couldn't delete duplicate layer")
		}
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't re-make overlay contents for %s", dir)
	}

	// now, as we do in convertAndOutput, we make a symlink for the hash
	// for each additional layer type so that they see the same data
	for _, desc := range descs[1:] {
		linkPath := overlayPath(config, desc.Digest)

		err = os.Symlink(target, linkPath)
		if err != nil {
			// as above, this symlink may already exist; if it does, we can
			// skip symlinking
			if !os.IsExist(err) {
				return false, errors.Wrapf(err, "couldn't symlink additional layer type")
			}

			// This sucks. Ideally, we'd be able to do:
			//
			// if fi, err := os.Lstat(linkPath); err == nil {
			// 	if fi.Mode()&os.ModeSymlink == 0 {
			// 		continue
			// 	}
			// 	existingLink, err := os.Readlink(linkPath)
			//
			// 	if err != nil {
			// 		return false, errors.Wrapf(err, "couldn't readlink %s", target)
			// 	}
			//
			// 	if existingLink != target {
			// 		return false, errors.Errorf("existing symlink %s doesn't point to %s (%s)", linkPath, target, existingLink)
			// 	}
			// }
			//
			// ...but we can't. Because umoci's tar generation
			// depends on golang maps which are randomized (e.g.
			// for when recording xattrs), it can generate tar
			// files with different hashes for the same directory.
			// So if this directory has already been repacked once,
			// we may have gotten a different hash for the same
			// thing.
			//
			// For now, we just punt and say "it's ok", but if
			// anyone ever implements GC() for overlay they'll want
			// to fix the assertion above.
			continue
		}

	}

	return true, nil
}

func repackOverlay(config types.StackerConfig, name string, layerTypes []types.LayerType) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	ovl, err := readOverlayMetadata(config, name)
	if err != nil {
		return err
	}

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

	log.Debugf("Generating overlay_dirs layers")
	mutated := false
	for i, layerType := range layerTypes {
		ods, ok := ovl.OverlayDirLayers[layerType]
		if !ok {
			continue
		}

		mutator := mutators[i]

		for _, od := range ods {
			now := time.Now()
			history := &ispec.History{
				Created:    &now,
				CreatedBy:  fmt.Sprintf("stacker overlay dir for %s", name),
				EmptyLayer: false,
			}

			err = mutator.AddExisting(context.Background(), od, history, od.Digest)
			if err != nil {
				return err
			}
		}
	}

	// generate blobs for each build layer
	for _, buildLayer := range ovl.BuiltLayers {

		didMutate, err := generateLayer(config, oci, mutators, buildLayer, layerTypes)
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

				ovl.Configs[layerType], err = mutators[i].Config(context.Background())
				if err != nil {
					return err
				}
			}
			ovl.HasBuiltOCIOutput = true
			err = ovl.write(config, buildLayer)
			if err != nil {
				return err
			}
		}
	}

	err = ovl.write(config, name)
	if err != nil {
		return err
	}

	didMutate, err := generateLayer(config, oci, mutators, name, layerTypes)
	if err != nil {
		return err
	}

	// if we didn't do anything, don't do anything :)
	if !didMutate && !mutated && len(ovl.OverlayDirLayers) == 0 {
		return nil
	}

	// if we did generate a layer for this, let's note that
	if didMutate {
		ovl.HasBuiltOCIOutput = true
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

		ovl.Configs[layerType], err = mutator.Config(context.Background())
		if err != nil {
			return err
		}

		err = oci.UpdateReference(context.Background(), layerType.LayerName(name), newPath.Root())
		if err != nil {
			return err
		}
	}

	return ovl.write(config, name)
}

func unpackOne(ociDir string, bundlePath string, digest digest.Digest, isSquashfs bool) error {
	if isSquashfs {
		return squashfs.ExtractSingleSquash(
			path.Join(ociDir, "blobs", "sha256", digest.Encoded()),
			bundlePath, "overlay")
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
