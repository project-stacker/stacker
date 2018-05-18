package stacker

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestLayerHashing(t *testing.T) {
	dir, err := ioutil.TempDir("", "stacker_cache_test")
	if err != nil {
		t.Fatalf("couldn't create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	config := StackerConfig{
		StackerDir: dir,
		RootFSDir:  dir,
	}

	layer := &Layer{
		From: &ImageSource{
			Type: "docker",
			Url:  "docker://centos:latest",
		},
		Run:       []string{"zomg"},
		BuildOnly: true,
	}

	sf := &Stackerfile{
		internal: map[string]*Layer{
			"foo": layer,
		},
	}

	cache, err := OpenCache(config, nil, sf)
	if err != nil {
		t.Fatalf("couldn't open cache %v", err)
	}

	// fake a successful build for a build-only layer
	err = os.MkdirAll(path.Join(dir, "foo"), 0755)
	if err != nil {
		t.Fatalf("couldn't fake successful bulid %v", err)
	}

	err = cache.Put("foo", ispec.Descriptor{})
	if err != nil {
		t.Fatalf("couldn't put to cache %v", err)
	}

	// change the layer, but look it up under the same name, to make sure
	// the layer itself is hashed
	layer.Run = []string{"jmh"}

	// ok, now re-load the persisted cache
	cache, err = OpenCache(config, nil, sf)
	if err != nil {
		t.Fatalf("couldn't re-load cache %v", err)
	}

	_, ok := cache.Lookup("foo")
	if ok {
		t.Errorf("found cached entry when I shouldn't have?")
	}
}
