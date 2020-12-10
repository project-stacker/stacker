package btrfs

import (
	"fmt"
	"path"

	"github.com/anuvu/stacker/container"
	"github.com/anuvu/stacker/lib"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/storage"
	"github.com/anuvu/stacker/types"
	"github.com/opencontainers/umoci"
	"github.com/pkg/errors"
)

func (b *btrfs) initEmptyLayer(name string, layerType types.LayerType) error {
	return container.RunInternalGoSubcommand(b.c, []string{
		"--tag", layerType.LayerName(name),
		"--oci-path", b.c.OCIDir,
		"--bundle-path", path.Join(b.c.RootFSDir, name),
		"init-empty",
	})
}

func determineLayerType(ociDir, tag string) (types.LayerType, error) {
	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return types.LayerType(""), err
	}
	defer oci.Close()

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return types.LayerType(""), err
	}

	return types.NewLayerTypeManifest(manifest)
}

func (b *btrfs) Repack(name string, layerTypes []types.LayerType, sfm types.StackerFiles) error {
	if len(layerTypes) != 1 {
		return errors.Errorf("btrfs backend does not support multiple layer types")
	}

	layerType := layerTypes[0]

	// first, let's copy whatever we can from wherever we can, either
	// import from the output if we already built a layer with this, or
	// import from the cache if nothing was ever built based on this
	baseTag, baseLayer, err := storage.FindFirstBaseInOutput(name, sfm)
	if err != nil {
		return err
	}

	initialized := false
	if baseLayer != nil {
		cacheDir := path.Join(b.c.StackerDir, "layer-bases", "oci")
		// if it's from a containers image import and the layer types match, just copy it to the output
		if types.IsContainersImageLayer(baseLayer.From.Type) {
			cacheTag, err := baseLayer.From.ParseTag()
			if err != nil {
				return err
			}

			sourceLayerType, err := determineLayerType(cacheDir, cacheTag)
			if err != nil {
				return err
			}
			if layerType == sourceLayerType {
				err = lib.ImageCopy(lib.ImageCopyOpts{
					Src:  fmt.Sprintf("oci:%s:%s", cacheDir, cacheTag),
					Dest: fmt.Sprintf("oci:%s:%s", b.c.OCIDir, layerType.LayerName(name)),
				})
				if err != nil {
					return err
				}
				initialized = true
			}
		} else {
			// otherwise if it's already been built and the base
			// types match, import it from there
			err = lib.ImageCopy(lib.ImageCopyOpts{
				Src:  fmt.Sprintf("oci:%s:%s", b.c.OCIDir, baseTag),
				Dest: fmt.Sprintf("oci:%s:%s", b.c.OCIDir, layerType.LayerName(name)),
			})
			if err != nil {
				return err
			}
			initialized = true
		}
	}

	if !initialized {
		if err = b.initEmptyLayer(name, layerType); err != nil {
			return err
		}
	}

	return container.RunInternalGoSubcommand(b.c, []string{
		"--oci-path", b.c.OCIDir,
		"--tag", name,
		"--bundle-path", path.Join(b.c.RootFSDir, name),
		"repack",
		"--layer-type", string(layerType),
	})
}
