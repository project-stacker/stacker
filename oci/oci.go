package oci

import (
	"context"
	"io"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
)

const (
	MediaTypeLayerSquashfs = "application/vnd.stacker.image.layer.squashfs"
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

// AddBlobNoCompression adds a blob to an OCI tag without compressing it (i.e.
// not through umoci.mutator).
func AddBlobNoCompression(oci casext.Engine, name string, content io.Reader) (ispec.Descriptor, error) {
	manifest, err := LookupManifest(oci, name)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	config, err := LookupConfig(oci, manifest.Config)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	blobDigest, blobSize, err := oci.PutBlob(context.Background(), content)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	desc := ispec.Descriptor{
		MediaType: MediaTypeLayerSquashfs,
		Digest:    blobDigest,
		Size:      blobSize,
	}

	manifest.Layers = append(manifest.Layers, desc)
	config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, blobDigest)

	configDigest, configSize, err := oci.PutBlobJSON(context.Background(), config)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	manifest.Config = ispec.Descriptor{
		MediaType: ispec.MediaTypeImageConfig,
		Digest:    configDigest,
		Size:      configSize,
	}

	manifestDigest, manifestSize, err := oci.PutBlobJSON(context.Background(), manifest)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	desc = ispec.Descriptor{
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
