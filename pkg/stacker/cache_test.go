package stacker

import (
	"os"
	"path"
	"testing"

	"github.com/mitchellh/hashstructure"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/stretchr/testify/assert"
	"stackerbuild.io/stacker/pkg/types"
)

func TestLayerHashing(t *testing.T) {
	dir, err := os.MkdirTemp("", "stacker_cache_test")
	if err != nil {
		t.Fatalf("couldn't create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	config := types.StackerConfig{
		StackerDir: dir,
		RootFSDir:  dir,
	}

	layerBases := path.Join(config.StackerDir, "layer-bases")
	err = os.MkdirAll(layerBases, 0755)
	if err != nil {
		t.Fatalf("couldn't mkdir for layer bases %v", err)
	}

	oci, err := umoci.CreateLayout(path.Join(layerBases, "oci"))
	if err != nil {
		t.Fatalf("couldn't creat OCI layout: %v", err)
	}
	defer oci.Close()

	err = umoci.NewImage(oci, "centos")
	if err != nil {
		t.Fatalf("couldn't create fake centos image %v", err)
	}

	stackerYaml := path.Join(dir, "stacker.yaml")
	err = os.WriteFile(stackerYaml, []byte(`
foo:
    from:
        type: docker
        url: docker://centos:latest
    run: zomg
    build_only: true
`), 0644)
	if err != nil {
		t.Fatalf("couldn't write stacker yaml %v", err)
	}

	sf, err := types.NewStackerfile(stackerYaml, false, nil)
	if err != nil {
		t.Fatalf("couldn't read stacker file %v", err)
	}

	cache, err := OpenCache(config, casext.Engine{}, types.StackerFiles{"dummy": sf})
	if err != nil {
		t.Fatalf("couldn't open cache %v", err)
	}

	// fake a successful build for a build-only layer
	err = os.MkdirAll(path.Join(dir, "foo"), 0755)
	if err != nil {
		t.Fatalf("couldn't fake successful bulid %v", err)
	}

	err = cache.Put("foo", map[types.LayerType]ispec.Descriptor{})
	if err != nil {
		t.Fatalf("couldn't put to cache %v", err)
	}

	// change the layer, but look it up under the same name, to make sure
	// the layer itself is hashed
	stackerYaml = path.Join(dir, "stacker.yaml")
	err = os.WriteFile(stackerYaml, []byte(`
foo:
    from:
        type: docker
        url: docker://centos:latest
    run: zomg meshuggah rocks
    build_only: true
`), 0644)
	if err != nil {
		t.Fatalf("couldn't write stacker yaml %v", err)
	}

	sf, err = types.NewStackerfile(stackerYaml, false, nil)
	if err != nil {
		t.Fatalf("couldn't read stacker file %v", err)
	}

	// ok, now re-load the persisted cache
	cache, err = OpenCache(config, casext.Engine{}, types.StackerFiles{"dummy": sf})
	if err != nil {
		t.Fatalf("couldn't re-load cache %v", err)
	}

	_, ok, err := cache.Lookup("foo")
	if err != nil {
		t.Errorf("lookup failed %v", err)
	}
	if ok {
		t.Errorf("found cached entry when I shouldn't have?")
	}
}

func TestCacheEntryChanged(t *testing.T) {
	assert := assert.New(t)

	h, err := hashstructure.Hash(CacheEntry{}, nil)
	assert.NoError(err)

	// The goal here is to make sure that the types of things in CacheEntry
	// haven't changed; if they have (aka this test fails), you should do
	// currentCacheVersion++, since stackers with an old cache will be
	// invalid with your current patch.
	//
	// This test works because the type information is included in the
	// hashstructure hash above, so using a zero valued CacheEntry is
	// enough to capture changes in types.
	assert.Equal(uint64(0x1ec739cbb7ee4e77), h)
}
