package oci

import (
	"context"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/log"
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

func hasDirEntries(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(ents) != 0
}

var tarEx sync.Mutex

// UnpackOne - unpack a single layer (Descriptor) found in ociDir to extractDir
//
//	The result of calling unpackOne is either error or the contents available
//	at the provided extractDir.  The extractDir should be either empty or
//	fully populated with this layer.
func UnpackOne(l ispec.Descriptor, ociDir string, extractDir string) error {
	// population of a dir is not atomic, at least for tar extraction.
	// As a result, we could hasDirEntries(extractDir) at the same time that
	// something is un-populating that dir due to a failed extraction (like
	// os.RemoveAll below).
	// There needs to be a lock on the extract dir (scoped to the overlay storage backend).
	// A sync.RWMutex would work well here since it is safe to check as long
	// as no one is populating or unpopulating.
	if hasDirEntries(extractDir) {
		// the directory was already populated.
		return nil
	}

	if squashfs.IsSquashfsMediaType(l.MediaType) {
		return squashfs.ExtractSingleSquash(
			path.Join(ociDir, "blobs", "sha256", l.Digest.Encoded()), extractDir)
	}
	switch l.MediaType {
	case ispec.MediaTypeImageLayer, ispec.MediaTypeImageLayerGzip:
		tarEx.Lock()
		defer tarEx.Unlock()

		oci, err := umoci.OpenLayout(ociDir)
		if err != nil {
			return err
		}
		defer oci.Close()

		compressed, err := oci.GetBlob(context.Background(), l.Digest)
		if err != nil {
			return err
		}
		defer compressed.Close()

		uncompressed, err := pgzip.NewReader(compressed)
		if err != nil {
			return err
		}

		err = layer.UnpackLayer(extractDir, uncompressed, nil)
		if err != nil {
			if rmErr := os.RemoveAll(extractDir); rmErr != nil {
				log.Errorf("Failed to remove dir '%s' after failed extraction: %v", extractDir, rmErr)
			}
		}
		return err
	}
	return errors.Errorf("unknown media type %s", l.MediaType)
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

	seen := map[digest.Digest]bool{}
	for _, curLayer := range manifest.Layers {
		// avoid calling UnpackOne twice for the same digest
		if seen[curLayer.Digest] {
			continue
		}
		seen[curLayer.Digest] = true

		// copy layer to avoid race on pool access.
		l := curLayer
		pool.Add(func(ctx context.Context) error {
			return UnpackOne(l, ociLayout, pathfunc(l.Digest))
		})
	}

	pool.DoneAddingJobs()

	err = pool.Run()
	if err != nil {
		return -1, err
	}

	return len(manifest.Layers), nil
}
