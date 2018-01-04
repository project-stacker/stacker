package stacker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/mitchellh/hashstructure"
	"github.com/openSUSE/umoci"
)

type BuildCache struct {
	oci     *umoci.Layout
	path    string
	Cache   map[string]umoci.Blob `json:"cache"`
	Version int                   `json:"version"`
}

func OpenCache(dir string, oci *umoci.Layout) (*BuildCache, error) {
	p := path.Join(dir, "build.cache")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &BuildCache{
				oci:     oci,
				path:    p,
				Cache:   map[string]umoci.Blob{},
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

func (c *BuildCache) Lookup(l *Layer) (umoci.Blob, bool) {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return umoci.Blob{}, false
	}

	result, ok := c.Cache[fmt.Sprintf("%d", h)]
	return result, ok
}

func (c *BuildCache) Put(l *Layer, blob umoci.Blob) error {
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
