package types

import (
	"fmt"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/project-stacker/stacker/squashfs"
)

type LayerType string

func NewLayerType(lt string) (LayerType, error) {
	switch lt {
	case "squashfs":
		fallthrough
	case "tar":
		return LayerType(lt), nil
	default:
		return LayerType(""), errors.Errorf("invalid layer type: %s", lt)
	}
}

func NewLayerTypeManifest(manifest ispec.Manifest) (LayerType, error) {
	if len(manifest.Layers) == 0 {
		return LayerType(""), errors.Errorf("no existing layers to determine layer type")
	}

	switch manifest.Layers[0].MediaType {
	case squashfs.BaseMediaTypeLayerSquashfs:
		// older stackers generated media types without compression information
		fallthrough
	case squashfs.GenerateSquashfsMediaType(squashfs.GzipCompression):
		fallthrough
	case squashfs.GenerateSquashfsMediaType(squashfs.ZstdCompression):
		return NewLayerType("squashfs")
	case ispec.MediaTypeImageLayerGzip:
		fallthrough
	case ispec.MediaTypeImageLayer:
		return NewLayerType("tar")
	default:
		return LayerType(""), errors.Errorf("invalid layer type %s", manifest.Layers[0].MediaType)
	}
}

func NewLayerTypes(lts []string) ([]LayerType, error) {
	ret := []LayerType{}
	for _, lt := range lts {
		hoisted, err := NewLayerType(lt)
		if err != nil {
			return nil, err
		}

		ret = append(ret, hoisted)
	}

	return ret, nil
}

func (lt LayerType) LayerName(tag string) string {
	if lt == "tar" {
		return tag
	}

	return fmt.Sprintf("%s-%s", tag, lt)
}
