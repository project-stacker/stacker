package stacker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/mitchellh/hashstructure"
	"github.com/openSUSE/umoci"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type BuildCache struct {
	oci     *umoci.Layout
	path    string
	Cache   map[string]ispec.Descriptor `json:"cache"`
	Version int                         `json:"version"`
}

func OpenCache(dir string, oci *umoci.Layout) (*BuildCache, error) {
	p := path.Join(dir, "build.cache")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &BuildCache{
				oci:     oci,
				path:    p,
				Cache:   map[string]ispec.Descriptor{},
				Version: 1,
			}, nil
		}
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	c := &BuildCache{}
	if err := json.Unmarshal(content, c); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *BuildCache) Lookup(l *Layer) (ispec.Descriptor, bool) {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return ispec.Descriptor{}, false
	}

	result, ok := c.Cache[fmt.Sprintf("%d", h)]
	return result, ok
}

func (c *BuildCache) Put(l *Layer, blob ispec.Descriptor) error {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return err
	}

	c.Cache[fmt.Sprintf("%d", h)] = blob
	return c.persist()
}

func (c *BuildCache) persist() error {
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.path, content, 0600)
}
