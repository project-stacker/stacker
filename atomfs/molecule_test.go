package atomfs

import (
	"fmt"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestAllowMissingVerityData(t *testing.T) {
	assert := assert.New(t)

	// no root hash annotations on this descriptor...
	const hash = "73cd1a9ab86defeb5e22151ceb96b347fc58b4318f64be05046c51d407a364eb"
	d := digest.NewDigestFromEncoded(digest.Algorithm("sha256"), hash)
	mol := Molecule{
		Atoms: []ispec.Descriptor{ispec.Descriptor{Digest: d}},
	}

	err := mol.mountUnderlyingAtoms()
	assert.NotNil(err)
	assert.Equal(fmt.Sprintf("sha256:%s is missing verity data", hash), err.Error())
}
