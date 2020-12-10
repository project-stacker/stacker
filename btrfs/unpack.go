package btrfs

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/anuvu/stacker/container"
	"github.com/anuvu/stacker/lib"
	"github.com/anuvu/stacker/log"
	stackeroci "github.com/anuvu/stacker/oci"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
)

func (b *btrfs) Unpack(tag, name string) error {
	oci, err := umoci.OpenLayout(b.c.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	cacheDir := path.Join(b.c.StackerDir, "layer-bases", "oci")
	cacheOCI, err := umoci.OpenLayout(cacheDir)
	if err != nil {
		return err
	}
	defer cacheOCI.Close()

	manifest, err := stackeroci.LookupManifest(cacheOCI, tag)
	if err != nil {
		return err
	}

	bundlePath := path.Join(b.c.RootFSDir, name)
	lastLayer, highestHash, err := b.findPreviousExtraction(cacheOCI, manifest)
	if err != nil {
		return err
	}

	dps, err := cacheOCI.ResolveReference(context.Background(), tag)
	if err != nil {
		return err
	}

	// restore whatever we already extracted
	if highestHash != "" {
		// Delete the previously created working snapshot; we're about
		// to create a new one.
		err = b.Delete(name)
		if err != nil {
			return err
		}

		err = b.Restore(highestHash, name)
		if err != nil {
			return err
		}
	}

	// if we're done, just prepare the metadata
	if lastLayer+1 == len(manifest.Layers) {
		err = prepareUmociMetadata(b, name, bundlePath, dps[0], highestHash)
		if err != nil {
			return err
		}
	} else {
		// otherwise, finish extracting
		startFrom := manifest.Layers[lastLayer+1]

		// again, if we restored from something that already been unpacked but
		// we're going to unpack stuff on top of it, we need to delete the old
		// metadata.
		err = cleanUmociMetadata(bundlePath)
		if err != nil {
			return err
		}

		err = container.RunInternalGoSubcommand(b.c, []string{
			"--oci-path", cacheDir,
			"--tag", tag,
			"--bundle-path", bundlePath,
			"unpack",
			"--start-from", startFrom.Digest.String(),
		})
		if err != nil {
			return err
		}

		// Ok, now that we have extracted and computed the mtree, let's
		// re-snapshot. The problem is that the snapshot in the callback won't
		// contain an mtree file, because the final mtree is generated after
		// the callback is called.
		hash, err := ComputeAggregateHash(manifest, manifest.Layers[len(manifest.Layers)-1])
		if err != nil {
			return err
		}
		err = b.Delete(hash)
		if err != nil {
			return err
		}

		err = b.Snapshot(name, hash)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *btrfs) findPreviousExtraction(oci casext.Engine, manifest ispec.Manifest) (int, string, error) {
	lastLayer := -1
	highestHash := ""
	for i, layerDesc := range manifest.Layers {
		hash, err := ComputeAggregateHash(manifest, layerDesc)
		if err != nil {
			return lastLayer, highestHash, err
		}

		if b.Exists(hash) {
			highestHash = hash
			lastLayer = i
			log.Debugf("found previous extraction of %s", layerDesc.Digest.String())
		} else {
			break
		}
	}

	return lastLayer, highestHash, nil
}

func prepareUmociMetadata(storage *btrfs, name string, bundlePath string, dp casext.DescriptorPath, highestHash string) error {
	// We need the mtree metadata to be present, but since these
	// intermediate snapshots were created after each layer was
	// extracted and the metadata wasn't, it won't necessarily
	// exist. We could create it at extract time, but that would
	// make everything really slow, since we'd have to walk the
	// whole FS after every layer which would probably slow things
	// way down.
	//
	// Instead, check to see if the metadata has been generated. If
	// it hasn't, we generate it, and then re-snapshot back (since
	// we can't write to the old snapshots) with the metadata.
	//
	// This means the first restore will be slower, but after that
	// it will be very fast.
	//
	// A further complication is that umoci metadata is stored in terms of
	// the manifest that corresponds to the layers. When a config changes
	// (or e.g. a manifest is updated to reflect new layers), the old
	// manifest will be unreferenced and eventually GC'd. However, the
	// underlying layers were the same, since the hash here is the
	// aggregate hash of only the bits in the layers, and not of anything
	// related to the manifest. Then, when some "older" build comes along
	// referencing these same layers but with a different manifest, we'll
	// fail.
	//
	// Since the manifest doesn't actually affect the bits on disk, we can
	// essentially just copy the old manifest over to whatever the new
	// manifest will be if the hashes don't match. We re-snapshot since
	// snapshotting is generally cheap and we assume that the "new"
	// manifest will be the default. However, this code will still be
	// triggered if we go back to the old manifest.
	mtreeName := strings.Replace(dp.Descriptor().Digest.String(), ":", "_", 1)
	_, err := os.Stat(path.Join(bundlePath, "umoci.json"))
	if err == nil {
		mtreePath := path.Join(bundlePath, mtreeName+".mtree")
		_, err := os.Stat(mtreePath)
		if err == nil {
			// The best case: this layer's mtree and metadata match
			// what we're currently trying to extract. Do nothing.
			return nil
		}

		// The mtree file didn't match. Find the other mtree (it must
		// exist) in this directory (since any are necessarily correct
		// per above) and move it to this mtree name, then regenerate
		// umoci's metadata.
		entries, err := ioutil.ReadDir(bundlePath)
		if err != nil {
			return err
		}

		generated := false
		for _, ent := range entries {
			if !strings.HasSuffix(ent.Name(), ".mtree") {
				continue
			}

			generated = true
			oldMtreePath := path.Join(bundlePath, ent.Name())
			err = lib.FileCopy(mtreePath, oldMtreePath)
			if err != nil {
				return err
			}

			os.RemoveAll(oldMtreePath)
			break
		}

		if !generated {
			return errors.Errorf("couldn't find old umoci metadata in %s", bundlePath)
		}
	} else {
		// Umoci's metadata wasn't present. Let's generate it.
		log.Infof("generating mtree metadata for snapshot (this may take a bit)...")
		err = container.RunInternalGoSubcommand(storage.c, []string{
			"--bundle-path", bundlePath,
			"generate-bundle-manifest",
			"--mtree-name", mtreeName,
		})
		if err != nil {
			return err
		}

	}

	meta := umoci.Meta{
		Version:    umoci.MetaVersion,
		MapOptions: layer.MapOptions{},
		From:       dp,
	}

	err = umoci.WriteBundleMeta(bundlePath, meta)
	if err != nil {
		return err
	}

	err = storage.Delete(highestHash)
	if err != nil {
		return err
	}

	err = storage.Snapshot(name, highestHash)
	if err != nil {
		return err
	}

	return nil
}

// clean all the umoci metadata (config.json for the OCI runtime, umoci.json
// for its metadata, anything named *.mtree)
func cleanUmociMetadata(bundlePath string) error {
	ents, err := ioutil.ReadDir(bundlePath)
	if err != nil {
		return err
	}

	for _, ent := range ents {
		if ent.Name() == "rootfs" {
			continue
		}

		os.Remove(path.Join(bundlePath, ent.Name()))
	}

	return nil
}
