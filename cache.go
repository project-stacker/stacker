package stacker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"

	"github.com/mitchellh/hashstructure"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/project-stacker/stacker/lib"
	"github.com/project-stacker/stacker/log"
	"github.com/project-stacker/stacker/types"
	"github.com/vbatts/go-mtree"
)

const currentCacheVersion = 12

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

type OverlayDirHash struct {
	Hash string
}

type CacheEntry struct {
	// A map of LayerType:Manifest this build corresponds to.
	Manifests map[types.LayerType]ispec.Descriptor

	// A map of the import url to the base64 encoded result of mtree walk
	// or sha256 sum of a file, depending on what Type is.
	Imports map[string]ImportHash

	// A map of the overlay_dir url to the base64 encoded result of mtree walk
	OverlayDirs map[string]OverlayDirHash

	// The name of this layer as it was built. Useful for the BuildOnly
	// case to make sure it still exists, and for printing error messages.
	Name string

	// The layer to cache
	Layer types.Layer

	// If the layer is of type "built", this is a hash of the base layer's
	// CacheEntry, which contains a hash of its imports. If there is a
	// mismatch with the current base layer's CacheEntry, the layer should
	// be rebuilt.
	Base string
}

type BuildCache struct {
	sfm     types.StackerFiles
	Cache   map[string]CacheEntry `json:"cache"`
	Version int                   `json:"version"`
	config  types.StackerConfig
}

type versionCheck struct {
	Version int `json:"version"`
}

func versionMatches(content []byte) (bool, error) {
	v := versionCheck{}
	if err := json.Unmarshal(content, &v); err != nil {
		return false, errors.Wrapf(err, "error parsing version")
	}

	return v.Version == currentCacheVersion, nil
}

func OpenCache(config types.StackerConfig, oci casext.Engine, sfm types.StackerFiles) (*BuildCache, error) {
	f, err := os.Open(config.CacheFile())
	cache := &BuildCache{
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

	cacheOk, err := versionMatches(content)
	if err != nil {
		return nil, err
	}

	if !cacheOk {
		log.Infof("old cache version found, clearing cache and rebuilding from scratch...")
		os.Remove(config.CacheFile())
		cache.Cache = map[string]CacheEntry{}
		cache.Version = currentCacheVersion
		return cache, nil
	}

	if err := json.Unmarshal(content, cache); err != nil {
		return nil, errors.Wrapf(err, "error parsing cache")
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
			for layerType, desc := range ent.Manifests {
				_, err = oci.FromDescriptor(context.Background(), desc)
				if err != nil {
					log.Infof("missing build for layer type %s", layerType)
					break
				}
			}
		}

		if err != nil {
			log.Infof("couldn't find %s, pruning it from the cache", ent.Name)
			log.Debugf("original error %s", err)
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

func (c *BuildCache) Lookup(name string) (*CacheEntry, bool, error) {
	l, ok := c.sfm.LookupLayerDefinition(name)

	if !ok {
		return nil, false, nil
	}

	result, ok := c.Cache[name]
	if !ok {
		// cache miss because the layer was not previously found. we
		// don't log a message here because it's probably not found
		// because it's either 1. the first time this thing has been
		// run or 2. a new layer from the previous run.
		return nil, false, nil
	}

	if !reflect.DeepEqual(result.Layer, l) {
		log.Debugf("cached: %+#v", result.Layer)
		log.Debugf("new: %+#v", l)
		log.Infof("cache miss because layer definition was changed")
		return nil, false, nil
	}

	baseHash, err := c.getBaseHash(name)
	if err != nil {
		return nil, false, err
	}

	if baseHash != result.Base {
		log.Infof("cache miss because base layer was changed")
		return nil, false, nil
	}

	for _, imp := range l.Imports {
		cachedImport, ok := result.Imports[imp.Path]
		if !ok {
			log.Infof("cache miss because of new import: %s", imp.Path)
			return nil, false, nil
		}

		fname := path.Base(imp.Path)
		importsDir := path.Join(c.config.StackerDir, "imports")
		diskPath := path.Join(importsDir, name, fname)
		st, err := os.Stat(diskPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Infof("cache miss because import was missing: %s", imp.Path)
				return nil, false, nil
			}
			return nil, false, err
		}

		if cachedImport.Type.IsDir() != st.IsDir() {
			log.Infof("cache miss because import type changed: %s", imp.Path)
			return nil, false, err
		}

		if st.IsDir() {
			dirChanged, err := isCachedDirChanged(diskPath, cachedImport.Hash)
			if err != nil {
				return nil, false, err
			}
			if dirChanged {
				log.Infof("cache miss because import dir content changed: %s", imp.Path)
				return nil, false, nil
			}
		} else {
			h, err := lib.HashFile(diskPath, true)
			if err != nil {
				return nil, false, err
			}

			if h != cachedImport.Hash {
				log.Infof("cache miss because import content changed: %s", imp.Path)
				return nil, false, nil
			}
		}
	}

	for _, overlayDir := range l.OverlayDirs {
		cachedOverlayDir, ok := result.OverlayDirs[overlayDir.Source]
		if !ok {
			log.Infof("cache miss because of new overlay_dir: %s", overlayDir.Source)
			return nil, false, nil
		}
		overlayDirDiskPath := path.Join(c.config.RootFSDir, name, "overlay_dirs", path.Base(overlayDir.Source), overlayDir.Dest)
		_, err := os.Stat(overlayDirDiskPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Infof("cache miss because overlay_dir was missing: %s", overlayDir.Source)
				return nil, false, nil
			}
			return nil, false, err
		}
		dirChanged, err := isCachedDirChanged(overlayDir.Source, cachedOverlayDir.Hash)
		if err != nil {
			return nil, false, err
		}
		if dirChanged {
			log.Infof("cache miss because overlay_dir content changed: %s", overlayDir.Source)
			return nil, false, nil
		}
	}

	return &result, true, nil
}

func isCachedDirChanged(dirPath string, cachedDirHash string) (bool, error) {
	rawCachedImport, err := base64.StdEncoding.DecodeString(cachedDirHash)
	if err != nil {
		return true, err
	}

	cachedDH, err := mtree.ParseSpec(bytes.NewBuffer(rawCachedImport))
	if err != nil {
		return true, err
	}

	dh, err := walkImport(dirPath)
	if err != nil {
		return true, err
	}

	diff, err := mtree.Compare(cachedDH, dh, mtreeKeywords)
	if err != nil {
		return true, err
	}

	if len(diff) > 0 {
		return true, nil
	}

	return false, nil
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
		return "", errors.Errorf("%s missing from stackerfile?", name)
	}

	switch l.From.Type {
	case types.BuiltLayer:
		// for built type, just use the hash of the cache entry
		baseEnt, ok, err := c.Lookup(l.From.Tag)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", errors.Errorf("couldn't find a cache of base layer for %s: %s", name, l.From.Tag)
		}

		baseHash, err := hashstructure.Hash(baseEnt, nil)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%d", baseHash), nil
	case types.TarLayer:
		// use the hash of the input tarball
		cacheDir := path.Join(c.config.StackerDir, "layer-bases")
		tar := path.Join(cacheDir, path.Base(l.From.Url))
		return lib.HashFile(tar, true)
	case types.OCILayer:
		fallthrough
	case types.DockerLayer:
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
		return "", errors.Errorf("unknown layer type: %v", l.From.Type)
	}
}

func (c *BuildCache) Put(name string, manifests map[types.LayerType]ispec.Descriptor) error {
	l, ok := c.sfm.LookupLayerDefinition(name)
	if !ok {
		return errors.Errorf("%s missing from stackerfile?", name)
	}

	baseHash, err := c.getBaseHash(name)
	if err != nil {
		return err
	}

	ent := CacheEntry{
		Manifests:   manifests,
		Imports:     map[string]ImportHash{},
		OverlayDirs: map[string]OverlayDirHash{},
		Name:        name,
		Layer:       l,
		Base:        baseHash,
	}

	for _, imp := range l.Imports {
		fname := path.Base(imp.Path)
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
			ih.Hash, err = lib.HashFile(diskPath, true)
			if err != nil {
				return err
			}
		}

		ent.Imports[imp.Path] = ih
	}

	for _, overlayDir := range l.OverlayDirs {
		odh := OverlayDirHash{}
		odh.Hash, err = getEncodedMtree(overlayDir.Source)
		if err != nil {
			return err
		}
		ent.OverlayDirs[overlayDir.Source] = odh
	}

	c.Cache[name] = ent
	return c.persist()
}

func (c *BuildCache) persist() error {
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.config.CacheFile(), content, 0600)
}
