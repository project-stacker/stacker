package erofs

import (
	"fmt"
	"strings"
)

type ErofsCompression string
type VerityMetadata bool

const (
	BaseMediaTypeLayerErofs = "application/vnd.stacker.image.layer.erofs"

	GzipCompression ErofsCompression = "gzip"
	ZstdCompression ErofsCompression = "zstd"

	veritySuffix = "verity"

	VerityMetadataPresent VerityMetadata = true
	VerityMetadataMissing VerityMetadata = false
)

func IsErofsMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, BaseMediaTypeLayerErofs)
}

func GenerateErofsMediaType(comp ErofsCompression, verity VerityMetadata) string {
	verityString := ""
	if verity {
		verityString = fmt.Sprintf("+%s", veritySuffix)
	}
	return fmt.Sprintf("%s+%s%s", BaseMediaTypeLayerErofs, comp, verityString)
}

func HasVerityMetadata(mediaType string) VerityMetadata {
	return VerityMetadata(strings.HasSuffix(mediaType, veritySuffix))
}
