package overlay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/log"
	stackeroci "stackerbuild.io/stacker/pkg/oci"
	"stackerbuild.io/stacker/pkg/types"
)

type overlayMetadata struct {
	Manifests map[types.LayerType]ispec.Manifest
	Configs   map[types.LayerType]ispec.Image

	// layers not yet rendered into the output image
	BuiltLayers []string

	// when this is true, that means that the layer has already been added
	// to the manifest, so Manifests.Layers[:-1] is the layer that
	// corresponds to what this built layer represents.
	HasBuiltOCIOutput bool

	// overlay_dir layers
	OverlayDirLayers map[types.LayerType][]ispec.Descriptor
}

func newOverlayMetadata() overlayMetadata {
	return overlayMetadata{
		Manifests: map[types.LayerType]ispec.Manifest{},
		Configs:   map[types.LayerType]ispec.Image{},
	}
}

func newOverlayMetadataFromOCI(oci casext.Engine, tag string) (overlayMetadata, error) {
	ovl := newOverlayMetadata()
	var err error

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return overlayMetadata{}, err
	}

	layerType, err := types.NewLayerTypeManifest(manifest)
	if err != nil {
		return overlayMetadata{}, err
	}

	ovl.Manifests[layerType] = manifest
	return ovl, nil
}

func readOverlayMetadata(config types.StackerConfig, tag string) (overlayMetadata, error) {
	metadataFile := path.Join(config.RootFSDir, tag, "overlay_metadata.json")
	content, err := os.ReadFile(metadataFile)
	if err != nil {
		return overlayMetadata{}, errors.Wrapf(err, "couldn't read overlay metadata %s", metadataFile)
	}

	var ovl overlayMetadata
	err = json.Unmarshal(content, &ovl)
	if err != nil {
		return overlayMetadata{}, errors.Wrapf(err, "couldnt' unmarshal overlay metadata %s", metadataFile)
	}

	if ovl.Manifests == nil {
		ovl.Manifests = map[types.LayerType]ispec.Manifest{}
	}

	if ovl.Configs == nil {
		ovl.Configs = map[types.LayerType]ispec.Image{}
	}

	return ovl, err
}

func (ovl overlayMetadata) write(config types.StackerConfig, tag string) error {
	content, err := json.Marshal(&ovl)
	if err != nil {
		return errors.Wrapf(err, "couldn't marshal overlay metadata")
	}
	metadataFile := path.Join(config.RootFSDir, tag, "overlay_metadata.json")
	err = os.WriteFile(metadataFile, content, 0644)
	if err != nil {
		return errors.Wrapf(err, "couldn't write overlay metadata %s", metadataFile)
	}

	return nil
}

func (ovl overlayMetadata) lxcRootfsString(config types.StackerConfig, tag string) (string, error) {
	// find *any* manifest to mount: we don't care if this is tar or
	// squashfs, we just need to mount something. the code that generates
	// the output needs to care about this, not this code.
	//
	// if there are no manifests (this came from a tar layer or whatever),
	// that's fine too; we just end up with two workaround directories as
	// below
	var manifest ispec.Manifest
	for _, m := range ovl.Manifests {
		manifest = m
		break
	}
	// same as above
	var descriptors []ispec.Descriptor
	for _, ds := range ovl.OverlayDirLayers {
		descriptors = ds
		break
	}

	lowerdirs := []string{}
	for _, layer := range manifest.Layers {
		contents := overlayPath(config, layer.Digest, "overlay")
		if _, err := os.Stat(contents); err != nil {
			return "", errors.Wrapf(err, "%s does not exist", contents)
		}
		lowerdirs = append(lowerdirs, contents)
	}

	for _, layer := range ovl.BuiltLayers {
		contents := path.Join(config.RootFSDir, layer, "overlay")
		if _, err := os.Stat(contents); err != nil {
			return "", errors.Wrapf(err, "%s does not exist", contents)
		}
		lowerdirs = append(lowerdirs, contents)
	}
	// mount overlay_dirs into lxc container
	for _, od := range descriptors {
		contents := overlayPath(config, od.Digest, "overlay")
		if _, err := os.Stat(contents); err != nil {
			return "", errors.Wrapf(err, "%s does not exist", contents)
		}
		lowerdirs = append(lowerdirs, contents)
	}

	// overlayfs doesn't work with < 2 lowerdirs, so we add some
	// workaround dirs if necessary (if e.g. the source only has
	// one layer, or it's an empty rootfs with no layers, we still
	// want an overlay mount to keep things consistent)
	for i := 0; i < 2-len(lowerdirs); i++ {
		workaround := path.Join(config.RootFSDir, tag, fmt.Sprintf("workaround%d", i))
		err := os.MkdirAll(workaround, 0755)
		if err != nil {
			return "", errors.Wrapf(err, "couldn't make workaround dir")
		}

		lowerdirs = append(lowerdirs, workaround)
	}

	// The OCI spec says that the first layer should be the bottom most
	// layer (i.e. the last layer in the manifest.Layers) list, and in
	// overlayfs it's the top most layer. So above, we've created this list
	// in exactly the backwards order. So, let's emit it to the args buffer
	// in reverse order.
	overlayArgs := bytes.NewBufferString("overlayfs:")
	for i := len(lowerdirs) - 1; i >= 0; i-- {
		overlayArgs.WriteString(lowerdirs[i])
		overlayArgs.WriteString(":")
	}

	// chop off the last : from lowerdir= above
	overlayArgs.Truncate(overlayArgs.Len() - 1)

	overlayArgs.WriteString(":")

	overlayArgs.WriteString(path.Join(config.RootFSDir, tag, "overlay"))

	log.Debugf("lxc rootfs overlay arg %s", overlayArgs.String())
	return overlayArgs.String(), nil
}
