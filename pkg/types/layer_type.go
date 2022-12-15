package types

import (
	"fmt"
	"strconv"
	"strings"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/squashfs"
)

type LayerType struct {
	Type   string
	Verity squashfs.VerityMetadata
}

func (lt LayerType) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s+%v", lt.Type, lt.Verity)), nil
}

func (lt *LayerType) UnmarshalText(text []byte) error {
	fields := strings.Split(string(text), "+")
	if len(fields) > 2 {
		return errors.Errorf("invalid layer type %s", string(text))
	}

	lt.Type = fields[0]
	if len(fields) == 1 {
		return nil
	}

	result, err := strconv.ParseBool(fields[1])
	if err != nil {
		return errors.Wrapf(err, "bad verity bool: %s", fields[1])
	}

	lt.Verity = squashfs.VerityMetadata(result)

	return nil
}

func NewLayerType(lt string, verity squashfs.VerityMetadata) (LayerType, error) {
	switch lt {
	case "squashfs":
		return LayerType{Type: lt, Verity: verity}, nil
	case "tar":
		return LayerType{Type: lt}, nil
	default:
		return LayerType{}, errors.Errorf("invalid layer type: %s", lt)
	}
}

func NewLayerTypeManifest(manifest ispec.Manifest) (LayerType, error) {
	if len(manifest.Layers) == 0 {
		return LayerType{}, errors.Errorf("no existing layers to determine layer type")
	}

	switch manifest.Layers[0].MediaType {
	case squashfs.BaseMediaTypeLayerSquashfs:
		// older stackers generated media types without compression information
		fallthrough
	case squashfs.GenerateSquashfsMediaType(squashfs.GzipCompression, squashfs.VerityMetadataMissing):
		fallthrough
	case squashfs.GenerateSquashfsMediaType(squashfs.ZstdCompression, squashfs.VerityMetadataMissing):
		return NewLayerType("squashfs", squashfs.VerityMetadataMissing)
	case squashfs.GenerateSquashfsMediaType(squashfs.GzipCompression, squashfs.VerityMetadataPresent):
		fallthrough
	case squashfs.GenerateSquashfsMediaType(squashfs.ZstdCompression, squashfs.VerityMetadataPresent):
		return NewLayerType("squashfs", squashfs.VerityMetadataPresent)
	case ispec.MediaTypeImageLayerGzip:
		fallthrough
	case ispec.MediaTypeImageLayer:
		return NewLayerType("tar", squashfs.VerityMetadataMissing)
	default:
		return LayerType{}, errors.Errorf("invalid layer type %s", manifest.Layers[0].MediaType)
	}
}

func NewLayerTypes(lts []string, verity squashfs.VerityMetadata) ([]LayerType, error) {
	ret := []LayerType{}
	for _, lt := range lts {
		hoisted, err := NewLayerType(lt, verity)
		if err != nil {
			return nil, err
		}

		ret = append(ret, hoisted)
	}

	return ret, nil
}

func (lt LayerType) LayerName(tag string) string {
	if lt.Type == "tar" {
		return tag
	}

	return fmt.Sprintf("%s-%s", tag, lt.Type)
}
