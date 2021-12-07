package btrfs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/anuvu/stacker/lib"
	stackermtree "github.com/anuvu/stacker/mtree"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/squashfs"
	"github.com/anuvu/stacker/storage"
	"github.com/anuvu/stacker/types"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/pkg/mtreefilter"
	"github.com/pkg/errors"
)

func (b *btrfs) initEmptyLayer(name string, layerType types.LayerType) error {
	var oci casext.Engine
	var err error

	tag := layerType.LayerName(name)
	ociDir := b.c.OCIDir
	bundlePath := path.Join(b.c.RootFSDir, name)

	if _, statErr := os.Stat(ociDir); statErr != nil {
		oci, err = umoci.CreateLayout(ociDir)
	} else {
		oci, err = umoci.OpenLayout(ociDir)
	}
	if err != nil {
		return errors.Wrapf(err, "Failed creating layout for %s", ociDir)
	}

	err = umoci.NewImage(oci, tag)
	if err != nil {
		return err
	}

	// kind of a hack, but the API won't let us init an empty image in a
	// bundle with data already in it, which is probably reasonable. so
	// what we do instead is: unpack the empty image above into a temp
	// directory, then copy the mtree/umoci metadata over to our rootfs.
	dir, err := ioutil.TempDir("", "umoci-init-empty")
	if err != nil {
		return errors.Wrapf(err, "couldn't create temp dir")
	}
	defer os.RemoveAll(dir)

	err = doUnpack(b.c, tag, ociDir, dir, "")
	if err != nil {
		return err
	}

	ents, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "couldn't read temp dir")
	}

	for _, ent := range ents {
		if ent.Name() == "rootfs" {
			continue
		}

		// copy all metadata to the real dir
		err = lib.FileCopy(path.Join(bundlePath, ent.Name()), path.Join(dir, ent.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

func determineLayerType(ociDir, tag string) (types.LayerType, error) {
	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return types.LayerType(""), err
	}
	defer oci.Close()

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return types.LayerType(""), err
	}

	return types.NewLayerTypeManifest(manifest)
}

func (b *btrfs) Repack(name string, layerTypes []types.LayerType, sfm types.StackerFiles) error {
	if len(layerTypes) != 1 {
		return errors.Errorf("btrfs backend does not support multiple layer types")
	}

	layerType := layerTypes[0]

	// first, let's copy whatever we can from wherever we can, either
	// import from the output if we already built a layer with this, or
	// import from the cache if nothing was ever built based on this
	baseTag, baseLayer, foundBase, err := storage.FindFirstBaseInOutput(name, sfm)
	if err != nil {
		return err
	}

	initialized := false
	if foundBase {
		cacheDir := path.Join(b.c.StackerDir, "layer-bases", "oci")
		// if it's from a containers image import and the layer types match, just copy it to the output
		if types.IsContainersImageLayer(baseLayer.From.Type) {
			cacheTag, err := baseLayer.From.ParseTag()
			if err != nil {
				return err
			}

			sourceLayerType, err := determineLayerType(cacheDir, cacheTag)
			if err != nil {
				return err
			}
			if layerType == sourceLayerType {
				err = lib.ImageCopy(lib.ImageCopyOpts{
					Src:  fmt.Sprintf("oci:%s:%s", cacheDir, cacheTag),
					Dest: fmt.Sprintf("oci:%s:%s", b.c.OCIDir, layerType.LayerName(name)),
				})
				if err != nil {
					return err
				}
				initialized = true
			}
		} else if !baseLayer.BuildOnly {
			// otherwise if it's already been built and the base
			// types match, import it from there
			err = lib.ImageCopy(lib.ImageCopyOpts{
				Src:  fmt.Sprintf("oci:%s:%s", b.c.OCIDir, layerType.LayerName(baseTag)),
				Dest: fmt.Sprintf("oci:%s:%s", b.c.OCIDir, layerType.LayerName(name)),
			})
			if err != nil {
				return err
			}
			initialized = true
		}
	}

	if !initialized {
		if err = b.initEmptyLayer(name, layerType); err != nil {
			return err
		}
	}

	return doRepack(name, b.c.OCIDir, path.Join(b.c.RootFSDir, name), layerType)
}

func doRepack(tag string, ociDir string, bundlePath string, layerType types.LayerType) error {
	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	meta, err := umoci.ReadBundleMeta(bundlePath)
	if err != nil {
		return err
	}

	mutator, err := mutate.New(oci, meta.From)
	if err != nil {
		return err
	}

	imageMeta, err := mutator.Meta(context.Background())
	if err != nil {
		return err
	}

	layerName := layerType.LayerName(tag)
	switch layerType {
	case "tar":
		now := time.Now()
		history := &ispec.History{
			Author:     imageMeta.Author,
			Created:    &now,
			CreatedBy:  "stacker umoci repack",
			EmptyLayer: false,
		}

		filters := []mtreefilter.FilterFunc{stackermtree.LayerGenerationIgnoreRoot}
		return umoci.Repack(oci, layerName, bundlePath, meta, history, filters, true, mutator)
	case "squashfs":
		return squashfs.GenerateSquashfsLayer(layerName, imageMeta.Author, bundlePath, ociDir, oci)
	default:
		return errors.Errorf("unknown layer type %s", layerType)
	}
}
