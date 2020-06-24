package overlay

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker/container"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/types"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
)

func overlayPath(config types.StackerConfig, d digest.Digest, subdir string) string {
	// dirs used in overlay lowerdir args can't have : in them, so lets
	// sanitize it
	safeName := strings.ReplaceAll(d.String(), ":", "_")
	return path.Join(config.RootFSDir, safeName, subdir)
}

func (o *overlay) Unpack(ociDir, tag, name string) error {
	oci, err := umoci.OpenLayout(ociDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	manifest, err := stackeroci.LookupManifest(oci, tag)
	if err != nil {
		return err
	}

	pool := NewThreadPool(runtime.NumCPU())

	for _, layer := range manifest.Layers {
		contents := overlayPath(o.config, layer.Digest, "overlay")
		switch layer.MediaType {
		case stackeroci.MediaTypeLayerSquashfs:
			// TODO: we can do something smart here, we don't even
			// need to unpack the layer, we can just mount it from
			// the import cache; but we'll need unpriv mount of
			// squashfs if we want to take advantage of that in our
			// builds. So maybe we should try to mount and extract
			// it if we can't? Anyway, punt for now.
			return errors.Errorf("squashfs + overlay not implemented")
		case ispec.MediaTypeImageLayer:
			fallthrough
		case ispec.MediaTypeImageLayerGzip:
			// don't extract things that have already been
			// extracted
			if _, err := os.Stat(contents); err == nil {
				continue
			}

			// TODO: when the umoci API grows support for uid
			// shifting, we can use the fancier features of context
			// cancelling in the thread pool...
			pool.Add(func(ctx context.Context) error {
				return container.RunUmociSubcommand(o.config, []string{
					"--tag", tag,
					"--oci-path", ociDir,
					"--bundle-path", contents,
					"unpack-one",
					"--digest", layer.Digest.String(),
				})
			})
		default:
			return errors.Errorf("unknown media type %s", layer.MediaType)
		}
	}

	pool.DoneAddingJobs()

	err = pool.Run()
	if err != nil {
		return err
	}

	err = o.Create(name)
	if err != nil {
		return err
	}

	ovl, err := newOverlayMetadata(oci, tag)
	if err != nil {
		return err
	}

	err = ovl.write(o.config, name)
	if err != nil {
		return err
	}

	return ovl.mount(o.config, name)
}

func (o *overlay) Repack(ociDir, name, layerType string) error {
	// this is really just a wrapper for the function below, RepackOverlay.
	// we just do this dance so it's run in a userns and the uids look
	// right.
	return container.RunUmociSubcommand(o.config, []string{
		"--tag", name,
		"--oci-path", ociDir,
		"repack-overlay",
	})
}

func generateLayer(mutator *mutate.Mutator, dir string) (bool, error) {
	ents, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, errors.Wrapf(err, "coudln't read overlay path %s", dir)
	}

	if len(ents) == 0 {
		return false, nil
	}

	// a hack, but GenerateInsertLayer() is the only thing that just takes
	// everything in a dir and makes it a layer.
	packOptions := layer.PackOptions{TranslateOverlayWhiteouts: true}
	uncompressed := layer.GenerateInsertLayer(dir, "/", false, &packOptions)
	defer uncompressed.Close()

	now := time.Now()
	history := &ispec.History{
		Created:    &now,
		CreatedBy:  "stacker umoci repack-overlay",
		EmptyLayer: false,
	}

	return true, mutator.Add(context.Background(), uncompressed, history)
}

func RepackOverlay(config types.StackerConfig, name string) error {
	oci, err := umoci.OpenLayout(config.OCIDir)
	if err != nil {
		return err
	}
	defer oci.Close()

	descPaths, err := oci.ResolveReference(context.Background(), name)
	if err != nil {
		return err
	}

	mutator, err := mutate.New(oci, descPaths[0])
	if err != nil {
		return errors.Wrapf(err, "mutator failed")
	}

	ovl, err := readOverlayMetadata(config, name)
	if err != nil {
		return err
	}

	mutated := false
	// generate blobs for each build layer
	for _, buildLayer := range ovl.BuiltLayers {
		didMutate, err := generateLayer(mutator, path.Join(config.RootFSDir, buildLayer, "overlay"))
		if err != nil {
			return err
		}
		if didMutate {
			mutated = true
		}
	}

	overlayPath := path.Join(config.RootFSDir, name, "overlay")
	didMutate, err := generateLayer(mutator, overlayPath)
	if err != nil {
		return err
	}
	if didMutate {
		mutated = true
	}

	// if we didn't do anything, don't do anything :)
	if !mutated {
		return nil
	}

	newPath, err := mutator.Commit(context.Background())
	if err != nil {
		return err
	}

	err = oci.UpdateReference(context.Background(), name, newPath.Root())
	if err != nil {
		return err
	}
	// TODO: right now we don't update the manifest in the overlay config,
	// even though we just generated one. we could do that, but there's no
	// extraction of the intermediate layer. we could do a copy to the new
	// intermediate hash that we generated, but that's expensive and maybe
	// useless. but maybe it's better than not sharing.
	//
	// the consequence is that for something like:
	//    a:
	//       from: docker://centos:latest
	//    b:
	//       from: a
	//    c:
	//       from: b
	//    d:
	//       from: b
	//
	// will cause the b layer to be generated twice in c and d, thus not
	// sharing it. not clear how much this scenario actually occrus in the
	// wild.
	return nil
}
