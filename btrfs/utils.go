package btrfs

import (
	"fmt"

	"github.com/minio/sha256-simd"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func ComputeAggregateHash(manifest ispec.Manifest, descriptor ispec.Descriptor) (string, error) {
	h := sha256.New()
	found := false

	for _, l := range manifest.Layers {
		_, err := h.Write([]byte(l.Digest.String()))
		if err != nil {
			return "", err
		}

		if l.Digest.String() == descriptor.Digest.String() {
			found = true
			break
		}
	}

	if !found {
		return "", errors.Errorf("couldn't find descriptor %s in manifest %s", descriptor.Digest.String(), manifest.Annotations["org.opencontainers.image.ref.name"])
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
