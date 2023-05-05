package oci

import (
	"context"
	"os"
	"path"
	"runtime"

	"github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/squashfs"
)

func LookupManifest(oci casext.Engine, tag string) (ispec.Manifest, error) {
	descriptorPaths, err := oci.ResolveReference(context.Background(), tag)
	if err != nil {
		return ispec.Manifest{}, err
	}

	if len(descriptorPaths) != 1 {
		return ispec.Manifest{}, errors.Errorf("bad descriptor %s", tag)
	}

	blob, err := oci.FromDescriptor(context.Background(), descriptorPaths[0].Descriptor())
	if err != nil {
		return ispec.Manifest{}, err
	}
	defer blob.Close()

	if blob.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return ispec.Manifest{}, errors.Errorf("descriptor does not point to a manifest: %s", blob.Descriptor.MediaType)
	}

	return blob.Data.(ispec.Manifest), nil
}

func LookupConfig(oci casext.Engine, desc ispec.Descriptor) (ispec.Image, error) {
	configBlob, err := oci.FromDescriptor(context.Background(), desc)
	if err != nil {
		return ispec.Image{}, err
	}

	if configBlob.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return ispec.Image{}, errors.Errorf("bad image config type: %s", configBlob.Descriptor.MediaType)
	}

	return configBlob.Data.(ispec.Image), nil

}

// UpdateImageConfig updates an oci tag with new config and new manifest
func UpdateImageConfig(oci casext.Engine, name string, newConfig ispec.Image, newManifest ispec.Manifest) (ispec.Descriptor, error) {
	configDigest, configSize, err := oci.PutBlobJSON(context.Background(), newConfig)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	newManifest.Config = ispec.Descriptor{
		MediaType: ispec.MediaTypeImageConfig,
		Digest:    configDigest,
		Size:      configSize,
	}

	manifestDigest, manifestSize, err := oci.PutBlobJSON(context.Background(), newManifest)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	desc := ispec.Descriptor{
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}

	err = oci.UpdateReference(context.Background(), name, desc)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	return desc, nil
}

func UnpackOne(ociDir string, bundlePath string, digest digest.Digest, isSquashfs bool) error {
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

// Unpack an image with "tag" from "ociLayout" into paths returned by "pathfunc"
func Unpack(ociLayout, tag string, pathfunc func(digest.Digest) string) (int, error) {
	oci, err := umoci.OpenLayout(ociLayout)
	if err != nil {
		return -1, err
	}
	defer oci.Close()

	manifest, err := LookupManifest(oci, tag)
	if err != nil {
		return -1, err
	}

	pool := NewThreadPool(runtime.NumCPU())

	for _, layer := range manifest.Layers {
		digest := layer.Digest
		contents := pathfunc(digest)
		if squashfs.IsSquashfsMediaType(layer.MediaType) {
			// don't really need to do this in parallel, but what
			// the hell.
			pool.Add(func(ctx context.Context) error {
				return UnpackOne(ociLayout, contents, digest, true)
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
					return UnpackOne(ociLayout, contents, digest, false)
				})
			default:
				return -1, errors.Errorf("unknown media type %s", layer.MediaType)
			}
		}
	}

	pool.DoneAddingJobs()

	return len(manifest.Layers), pool.Run()
}
