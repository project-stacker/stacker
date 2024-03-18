package stacker

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/klauspost/pgzip"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/lib"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

type BaseLayerOpts struct {
	Config     types.StackerConfig
	Name       string
	Layer      types.Layer
	Cache      *BuildCache
	OCI        casext.Engine
	LayerTypes []types.LayerType
	Storage    types.Storage
	Progress   bool
}

// GetBase grabs the base layer and puts it in the cache.
func GetBase(o BaseLayerOpts) error {
	switch o.Layer.From.Type {
	case types.BuiltLayer:
		fallthrough
	case types.ScratchLayer:
		return nil
	case types.TarLayer:
		cacheDir := path.Join(o.Config.StackerDir, "layer-bases")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return err
		}

		_, err := acquireUrl(o.Config, o.Storage, o.Layer.From.Url, cacheDir, "", "", nil, -1, -1, o.Progress)
		return err
	/* now we can do all the containers/image types */
	case types.OCILayer:
		fallthrough
	case types.DockerLayer:
		err := importContainersImage(o.Layer.From, o.Config, o.Progress)
		if o.Layer.Bom != nil && o.Layer.Bom.Generate && (o.Layer.From.Type == types.DockerLayer) {
			bomPath := path.Join(o.Config.StackerDir, "artifacts", o.Name)
			err = getArtifact(bomPath, "application/spdx+json", o.Layer.From.Url, "", "", o.Layer.From.Insecure)
			if err != nil {
				log.Errorf("sbom for image %s not found", o.Layer.From.Url)
			}
		}
		return err
	default:
		return errors.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

// SetupRootfs assumes the base layer is correct in the cache, and sets up the
// base to the output.
//
// If the layer is a build only layer, this code simply initializes the
// filesystem in roots to the built tag's filesystem.
func SetupRootfs(o BaseLayerOpts) error {
	err := o.Storage.Delete(o.Name)
	if err != nil && !os.IsNotExist(errors.Unwrap(err)) {
		return err
	}

	if o.Layer.From.Type == types.BuiltLayer {
		// For built type images, we already have the base fs content
		// and umoci metadata. So let's just use that.
		return o.Storage.Restore(o.Layer.From.Tag, o.Name)
	}

	// For everything else, we create a new snapshot and extract whatever
	// we can on top of it.
	if err := o.Storage.Create(o.Name); err != nil {
		return err
	}

	switch o.Layer.From.Type {
	case types.ScratchLayer:
		return o.Storage.SetupEmptyRootfs(o.Name)
	case types.TarLayer:
		err := o.Storage.SetupEmptyRootfs(o.Name)
		if err != nil {
			return err
		}
		return setupTarRootfs(o)
	case types.OCILayer:
		fallthrough
	case types.DockerLayer:
		err := setupContainersImageRootfs(o)
		if err != nil && errors.Is(err, types.ErrEmptyLayers) {
			return o.Storage.SetupEmptyRootfs(o.Name)
		}
		return err
	default:
		return errors.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

func importContainersImage(is types.ImageSource, config types.StackerConfig, progress bool) error {
	toImport, err := is.ContainersImageURL()
	if err != nil {
		return err
	}

	tag, err := is.ParseTag()
	if err != nil {
		return err
	}

	// Note that we can do this over the top of the cache every time, since
	// skopeo should be smart enough to only copy layers that have changed.
	cacheDir := path.Join(config.StackerDir, "layer-bases", "oci")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	defer func() {
		oci, err := umoci.OpenLayout(cacheDir)
		if err != nil {
			// Some error might have occurred, in which case we
			// don't have a valid OCI layout, which is fine.
			return
		}
		defer oci.Close()
	}()

	var progressWriter io.Writer
	if progress {
		progressWriter = os.Stderr
	}

	log.Infof("loading %s", toImport)
	err = lib.ImageCopy(lib.ImageCopyOpts{
		Src:        toImport,
		Dest:       fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		SrcSkipTLS: is.Insecure,
		Progress:   progressWriter,
	})
	if err != nil {
		return errors.Wrapf(err, "couldn't import base layer %s", tag)
	}

	return err
}

func setupContainersImageRootfs(o BaseLayerOpts) error {
	target := path.Join(o.Config.RootFSDir, o.Name)
	log.Debugf("unpacking to %s", target)

	cacheTag, err := o.Layer.From.ParseTag()
	if err != nil {
		return err
	}

	return o.Storage.Unpack(cacheTag, o.Name)
}

func setupTarRootfs(o BaseLayerOpts) error {
	// initialize an empty image, then extract it
	cacheDir := path.Join(o.Config.StackerDir, "layer-bases")
	tar := path.Join(cacheDir, path.Base(o.Layer.From.Url))

	layerPath := o.Storage.TarExtractLocation(o.Name)
	return unpackTar(layerPath, tar)
}

func unpackTar(destDir string, tar string) error {
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
