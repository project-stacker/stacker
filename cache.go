package stacker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/mitchellh/hashstructure"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/vbatts/go-mtree"
)

const currentCacheVersion = 7

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

	// The name of this layer as it was built. Useful for the BuildOnly
	// case to make sure it still exists, and for printing error messages.
	Name string

	// The layer to cache
	Layer *Layer

	// If the layer is of type "built", this is a hash of the base layer's
	// CacheEntry, which contains a hash of its imports. If there is a
	// mismatch with the current base layer's CacheEntry, the layer should
	// be rebuilt.
	Base string
}

type BuildCache struct {
	path    string
	sfm     StackerFiles
	Cache   map[string]CacheEntry `json:"cache"`
	Version int                   `json:"version"`
	config  StackerConfig
}

func OpenCache(config StackerConfig, oci casext.Engine, sfm StackerFiles) (*BuildCache, error) {
	p := path.Join(config.StackerDir, "build.cache")
	f, err := os.Open(p)
	cache := &BuildCache{
		path:   p,
		sfm:    sfm,
		config: config,
	}

	if err != nil {
		if os.IsNotExist(err) {
			cache.Cache = map[string]CacheEntry{}
			cache.Version = currentCacheVersion
			return cache, nil
		}
		return nil, err
	}

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(content, cache); err != nil {
		return nil, err
	}

	if cache.Version != currentCacheVersion {
		fmt.Println("old cache version found, clearing cache and rebuilding from scratch...")
		os.Remove(p)
		cache.Cache = map[string]CacheEntry{}
		cache.Version = currentCacheVersion
		return cache, nil
	}

	pruned := false
	for hash, ent := range cache.Cache {
		if ent.Layer.BuildOnly {
			// If this is a build only layer, we just rely on the
			// fact that it's in the rootfs dir (and hope that
			// nobody has touched it). So, let's stat its dir and
			// keep going.
			_, err = os.Stat(path.Join(config.RootFSDir, ent.Name))
		} else {
			_, err = oci.FromDescriptor(context.Background(), ent.Blob)
		}

		if err != nil {
			fmt.Printf("couldn't find %s, pruning it from the cache\n", ent.Name)
			delete(cache.Cache, hash)
			pruned = true
		}
	}

	if pruned {
		err := cache.persist()
		if err != nil {
			return nil, err
		}
	}

	return cache, nil
}

/* Explicitly don't use mtime */
var mtreeKeywords = []mtree.Keyword{"type", "link", "uid", "gid", "xattr", "mode", "sha256digest"}

func walkImport(path string) (*mtree.DirectoryHierarchy, error) {
	return mtree.Walk(path, nil, mtreeKeywords, nil)
}

func hashFile(path string, includeMode bool) (string, error) {
	h := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}

	if includeMode {
		// Include file mode when computing the hash
		// In general we want to do this, but not all external
		// tooling includes it, so we can't compare it with the hash
		// in the reply of a HTTP HEAD call

		fi, err := f.Stat()
		if err != nil {
			return "", err
		}

		_, err = h.Write([]byte(fmt.Sprintf("%v", fi.Mode())))
		if err != nil {
			return "", errors.Wrapf(err, "couldn't write mode")
		}
	}

	d := digest.NewDigest("sha256", h)
	return d.String(), nil
}

func (c *BuildCache) Lookup(name string) (*CacheEntry, bool) {
	l, ok := c.sfm.LookupLayerDefinition(name)
	if !ok {
		return nil, false
	}

	result, ok := c.Cache[name]
	if !ok {
		return nil, false
	}

	h1, err := hashstructure.Hash(result.Layer, nil)
	if err != nil {
		return nil, false
	}

	h2, err := hashstructure.Hash(l, nil)
	if err != nil {
		return nil, false
	}

	if h1 != h2 {
		fmt.Println("cache miss because layer definition was changed")
		return nil, false
	}

	baseHash, err := c.getBaseHash(name)
	if err != nil {
		return nil, false
	}

	if baseHash != result.Base {
		fmt.Println("cache miss because base layer was changed")
		return nil, false
	}

	imports, err := l.ParseImport()
	if err != nil {
		return nil, false
	}

	for _, imp := range imports {
		cachedImport, ok := result.Imports[imp]
		if !ok {
			fmt.Println("cache miss because of new import: ", imp)
			return nil, false
		}

		fname := path.Base(imp)
		importsDir := path.Join(c.config.StackerDir, "imports")
		diskPath := path.Join(importsDir, name, fname)
		st, err := os.Stat(diskPath)
		if err != nil {
			return nil, false
		}

		if cachedImport.Type.IsDir() != st.IsDir() {
			fmt.Println("cache miss because import type changed: ", imp)
			return nil, false
		}

		if st.IsDir() {
			rawCachedImport, err := base64.StdEncoding.DecodeString(cachedImport.Hash)
			if err != nil {
				return nil, false
			}

			cachedDH, err := mtree.ParseSpec(bytes.NewBuffer(rawCachedImport))
			if err != nil {
				return nil, false
			}

			dh, err := walkImport(diskPath)
			if err != nil {
				return nil, false
			}

			diff, err := mtree.Compare(cachedDH, dh, mtreeKeywords)
			if err != nil {
				return nil, false
			}

			if len(diff) > 0 {
				fmt.Println("cache miss because import dir content changed: ", imp)
				return nil, false
			}
		} else {
			h, err := hashFile(diskPath, true)
			if err != nil {
				return nil, false
			}

			if h != cachedImport.Hash {
				fmt.Println("cache miss because import content changed: ", imp)
				return nil, false
			}
		}
	}

	return &result, true
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

// getBaseHash returns some kind of "hash" for the base layer, whatever type it
// may be.
func (c *BuildCache) getBaseHash(name string) (string, error) {
	l, ok := c.sfm.LookupLayerDefinition(name)
	if !ok {
		return "", fmt.Errorf("%s missing from stackerfile?", name)
	}

	switch l.From.Type {
	case BuiltType:
		// for built type, just use the hash of the cache entry
		baseEnt, ok := c.Lookup(l.From.Tag)
		if !ok {
			return "", fmt.Errorf("couldn't find a cache of base layer")
		}

		baseHash, err := hashstructure.Hash(baseEnt, nil)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%d", baseHash), nil
	case ScratchType:
		// no base, no hash :)
		return "", nil
	case TarType:
		// use the hash of the input tarball
		cacheDir := path.Join(c.config.StackerDir, "layer-bases")
		tar := path.Join(cacheDir, path.Base(l.From.Url))
		return hashFile(tar, true)
	case OCIType:
		fallthrough
	case DockerType:
		fallthrough
	case ZotType:
		tag, err := l.From.ParseTag()
		if err != nil {
			return "", err
		}

		// use the manifest hash of the thing in the cache
		oci, err := umoci.OpenLayout(path.Join(c.config.StackerDir, "layer-bases", "oci"))
		if err != nil {
			return "", err
		}
		defer oci.Close()

		descriptorPaths, err := oci.ResolveReference(context.Background(), tag)
		if err != nil {
			return "", err
		}

		if len(descriptorPaths) != 1 {
			return "", errors.Errorf("duplicate manifests for %s", tag)
		}

		return descriptorPaths[0].Descriptor().Digest.Encoded(), nil
	default:
		return "", fmt.Errorf("unknown layer type: %v", l.From.Type)
	}
}

func (c *BuildCache) Put(name string, blob ispec.Descriptor) error {
	l, ok := c.sfm.LookupLayerDefinition(name)
	if !ok {
		return fmt.Errorf("%s missing from stackerfile?", name)
	}

	baseHash, err := c.getBaseHash(name)
	if err != nil {
		return err
	}

	ent := CacheEntry{
		Blob:    blob,
		Imports: map[string]ImportHash{},
		Name:    name,
		Layer:   l,
		Base:    baseHash,
	}

	imports, err := l.ParseImport()
	if err != nil {
		return err
	}

	for _, imp := range imports {
		fname := path.Base(imp)
		importsDir := path.Join(c.config.StackerDir, "imports")
		diskPath := path.Join(importsDir, name, fname)
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
			ih.Hash, err = hashFile(diskPath, true)
			if err != nil {
				return err
			}
		}

		ent.Imports[imp] = ih
	}

	c.Cache[name] = ent
	return c.persist()
}

func (c *BuildCache) persist() error {
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.path, content, 0600)
}
