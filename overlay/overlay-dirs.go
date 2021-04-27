package overlay

import (
	"os"
	"path"

	"github.com/anuvu/stacker/lib"
	"github.com/anuvu/stacker/types"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/pkg/errors"
)

// generateOverlayDirsLayers generates oci layers from all overlay_dirs of this image
// and saves the layer descriptors in the overlay_metadata.json
func generateOverlayDirsLayers(name string, layerTypes []types.LayerType, overlayDirs types.OverlayDirs, config types.StackerConfig) error {
	ovl, err := readOverlayMetadata(config, name)
	if err != nil {
		return err
	}
	ovl.OverlayDirLayers = make(map[types.LayerType][]ispec.Descriptor, 1)
	for _, layerType := range layerTypes {
		for _, od := range overlayDirs {
			desc, err := generateOverlayDirLayer(name, layerType, od, config)
			if err != nil {
				return err
			}
			ovl.OverlayDirLayers[layerType] = append(ovl.OverlayDirLayers[layerType], desc)
		}
	}
	err = ovl.write(config, name)
	if err != nil {
		return err
	}

	return nil
}

// generateOverlayDirLayer generates an oci layer from one overlay_dir
func generateOverlayDirLayer(name string, layerType types.LayerType, overlayDir types.OverlayDir, config types.StackerConfig) (ispec.Descriptor, error) {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return ispec.Descriptor{}, err
	}
	defer oci.Close()

	contents := path.Join(config.RootFSDir, name, "overlay_dirs", path.Base(overlayDir.Source))
	blob, err := generateBlob(layerType, contents, config.OCIDir)
	if err != nil {
		return ispec.Descriptor{}, err
	}
	defer blob.Close()

	desc, err := ociPutBlob(blob, config, layerType)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	if err = os.MkdirAll(overlayPath(config, desc.Digest), 0755); err != nil {
		return ispec.Descriptor{}, err
	}

	err = os.Symlink(contents, overlayPath(config, desc.Digest, "overlay"))
	if err != nil {
		return ispec.Descriptor{}, errors.Wrapf(err, "failed to create symlink")
	}

	return desc, nil
}

// copyOverlayDirs copies each overlay_dir into container 'name' rootfs
// later they will be outputted as layers by using generateOverlayDirLayer
func copyOverlayDirs(name string, overlayDirs types.OverlayDirs, rootfs string) error {
	for _, overlayDir := range overlayDirs {
		st, err := os.Stat(overlayDir.Source)
		if os.IsNotExist(err) {
			return errors.Errorf("Given overlay_dir %s doesn't exists", overlayDir.Source)
		}
		if !st.IsDir() {
			return errors.Errorf("Given overlay_dir %s should be a directory", overlayDir.Source)
		}
		contents := path.Join(rootfs, name, "overlay_dirs", path.Base(overlayDir.Source), overlayDir.Dest)
		if _, err := os.Stat(contents); os.IsNotExist(err) {
			if err = os.MkdirAll(contents, 0755); err != nil {
				return err
			}
		}
		if err := lib.DirCopy(contents, overlayDir.Source); err != nil {
			return err
		}
	}
	return nil
}
