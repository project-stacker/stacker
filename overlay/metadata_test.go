package overlay

import (
	"testing"

	"github.com/mitchellh/hashstructure"
	"github.com/stretchr/testify/assert"
)

func TestOverlayMetadataChanged(t *testing.T) {
	assert := assert.New(t)

	// see TestCacheEntryChanged for a full explanation, but if you need to
	// bump this, you should bump the cache version as well since things
	// may not be transferrable across versions.
	h, err := hashstructure.Hash(overlayMetadata{}, nil)
	assert.NoError(err)

	assert.Equal(uint64(0x64bca1f8d0e31be2), h)
}
