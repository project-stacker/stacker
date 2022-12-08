package squashfs

import (
	"fmt"
	"strings"
)

type SquashfsCompression string
type VerityMetadata bool

const (
	BaseMediaTypeLayerSquashfs = "application/vnd.stacker.image.layer.squashfs"

	GzipCompression SquashfsCompression = "gzip"
	ZstdCompression SquashfsCompression = "zstd"

	veritySuffix = "verity"

	VerityMetadataPresent VerityMetadata = true
	VerityMetadataMissing VerityMetadata = false
)

func IsSquashfsMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, BaseMediaTypeLayerSquashfs)
}

func GenerateSquashfsMediaType(comp SquashfsCompression, verity VerityMetadata) string {
	verityString := ""
	if verity {
		verityString = fmt.Sprintf("+%s", veritySuffix)
	}
	return fmt.Sprintf("%s+%s%s", BaseMediaTypeLayerSquashfs, comp, verityString)
}

func HasVerityMetadata(mediaType string) VerityMetadata {
	return VerityMetadata(strings.HasSuffix(mediaType, veritySuffix))
}
