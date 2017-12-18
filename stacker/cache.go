package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/anuvu/stacker"
	"github.com/mitchellh/hashstructure"
)

type cache struct {
	path    string                 `json:omit`
	Cache   map[uint64]LayerInfo   `json:"cache"`
	Version int                    `json:"version"`
}

func openCache(sc stacker.StackerConfig) (*cache, error) {
	p := path.Join(sc.StackerDir, "build.cache")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &cache{
				path:    p,
				Cache:   map[uint64]LayerInfo{},
				Version: 1,
			}, nil
		}
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	c := &cache{}
	if err := json.Unmarshal(content, c); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *cache) Lookup(l *stacker.Layer) (LayerInfo, bool) {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return LayerInfo{}, false
	}
	result, ok := c.Cache[h]
	return result, ok
}

func (c *cache) Put(l *stacker.Layer, blob LayerInfo) error {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return err
	}

	c.Cache[h] = blob
	return c.persist()
}

func (c *cache) persist() error {
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.path, content, 0600)
}
