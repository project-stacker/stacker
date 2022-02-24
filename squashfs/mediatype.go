package squashfs

import (
	"fmt"
	"strings"
)

type SquashfsCompression string

const (
	BaseMediaTypeLayerSquashfs = "application/vnd.stacker.image.layer.squashfs"

	GzipCompression SquashfsCompression = "gzip"
	ZstdCompression SquashfsCompression = "zstd"
)

func IsSquashfsMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, BaseMediaTypeLayerSquashfs)
}

func GenerateSquashfsMediaType(comp SquashfsCompression) string {
	return fmt.Sprintf("%s+%s", BaseMediaTypeLayerSquashfs, comp)
}
