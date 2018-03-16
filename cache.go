package stacker

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/mitchellh/hashstructure"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vbatts/go-mtree"
)

const currentCacheVersion = 2

type ImportType int

const (
	ImportFile ImportType = iota
	ImportDir  ImportType = iota
)

func (it ImportType) IsDir() bool {
	return ImportDir == it
}

type ImportHash struct {
	// Unfortuantely, mtree doesn't work if you just pass it a single file,
	// so we use the sha256sum of the file, or the mtree encoding if it's a
	// directory. This indicates which.
	Type ImportType
	Hash string
}

type CacheEntry struct {
	// The manifest that this corresponds to.
	Blob ispec.Descriptor

	// A map of the import url to the base64 encoded result of mtree walk
	// or sha256 sum of a file, depending on what Type is.
	Imports map[string]ImportHash
}

type BuildCache struct {
	path    string
	Cache   map[string]CacheEntry `json:"cache"`
	Version int                   `json:"version"`
}

func OpenCache(dir string) (*BuildCache, error) {
	p := path.Join(dir, "build.cache")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &BuildCache{
				path:    p,
				Cache:   map[string]CacheEntry{},
				Version: currentCacheVersion,
			}, nil
		}
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	cache := &BuildCache{path: p}
	if err := json.Unmarshal(content, cache); err != nil {
		return nil, err
	}

	if cache.Version != currentCacheVersion {
		fmt.Println("old cache version not found, clearing cache and rebuilding from scratch...")
		os.Remove(p)
		return &BuildCache{
			path:    p,
			Cache:   map[string]CacheEntry{},
			Version: currentCacheVersion,
		}, nil
	}

	return cache, nil
}

/* Explicitly don't use mtime */
var mtreeKeywords = []mtree.Keyword{"type", "link", "uid", "gid", "xattr", "mode", "sha256digest"}

func walkImport(path string) (*mtree.DirectoryHierarchy, error) {
	return mtree.Walk(path, nil, mtreeKeywords, nil)
}

func hashFile(path string) (string, error) {
	h := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(h, f)
	f.Close()
	if err != nil {
		return "", err
	}

	d := digest.NewDigest("sha256", h)
	return d.String(), nil
}

func (c *BuildCache) Lookup(l *Layer, importsDir string) (ispec.Descriptor, bool) {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return ispec.Descriptor{}, false
	}

	result, ok := c.Cache[fmt.Sprintf("%d", h)]
	if !ok {
		return ispec.Descriptor{}, false
	}

	imports, err := l.ParseImport()
	if err != nil {
		return ispec.Descriptor{}, false
	}

	for _, imp := range imports {
		name := path.Base(imp)
		cachedImport, ok := result.Imports[name]
		if !ok {
			return ispec.Descriptor{}, false
		}

		diskPath := path.Join(importsDir, name)
		st, err := os.Stat(diskPath)
		if err != nil {
			return ispec.Descriptor{}, false
		}

		if cachedImport.Type.IsDir() != st.IsDir() {
			return ispec.Descriptor{}, false
		}

		if st.IsDir() {
			rawCachedImport, err := base64.StdEncoding.DecodeString(cachedImport.Hash)
			if err != nil {
				return ispec.Descriptor{}, false
			}

			cachedDH, err := mtree.ParseSpec(bytes.NewBuffer(rawCachedImport))
			if err != nil {
				return ispec.Descriptor{}, false
			}

			dh, err := walkImport(diskPath)
			if err != nil {
				return ispec.Descriptor{}, false
			}

			diff, err := mtree.Compare(cachedDH, dh, mtreeKeywords)
			if err != nil {
				return ispec.Descriptor{}, false
			}

			if len(diff) > 0 {
				return ispec.Descriptor{}, false
			}
		} else {
			h, err := hashFile(diskPath)
			if err != nil {
				return ispec.Descriptor{}, false
			}

			if h != cachedImport.Hash {
				return ispec.Descriptor{}, false
			}
		}
	}

	return result.Blob, true
}

func getEncodedMtree(path string) (string, error) {
	dh, err := walkImport(path)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	_, err = dh.WriteTo(buf)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (c *BuildCache) Put(l *Layer, importsDir string, blob ispec.Descriptor) error {
	h, err := hashstructure.Hash(l, nil)
	if err != nil {
		return err
	}

	ent := CacheEntry{
		Blob:    blob,
		Imports: map[string]ImportHash{},
	}

	imports, err := l.ParseImport()
	if err != nil {
		return err
	}

	for _, imp := range imports {
		name := path.Base(imp)
		diskPath := path.Join(importsDir, name)
		st, err := os.Stat(diskPath)
		if err != nil {
			return err
		}

		ih := ImportHash{}
		if st.IsDir() {
			ih.Type = ImportDir
			ih.Hash, err = getEncodedMtree(diskPath)
			if err != nil {
				return err
			}
		} else {
			ih.Type = ImportFile
			ih.Hash, err = hashFile(diskPath)
			if err != nil {
				return err
			}
		}

		ent.Imports[name] = ih
	}

	c.Cache[fmt.Sprintf("%d", h)] = ent
	return c.persist()
}

func (c *BuildCache) persist() error {
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.path, content, 0600)
}
