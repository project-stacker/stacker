package overlay

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/anuvu/stacker/container"
	"github.com/anuvu/stacker/lib"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/types"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func safeOverlayName(d digest.Digest) string {
	// dirs used in overlay lowerdir args can't have : in them, so lets
	// sanitize it
	return strings.ReplaceAll(d.String(), ":", "_")
}

func overlayPath(config types.StackerConfig, d digest.Digest, subdirs ...string) string {
	safeName := safeOverlayName(d)
	dirs := []string{config.RootFSDir, safeName}
	dirs = append(dirs, subdirs...)
	return path.Join(dirs...)
}

func (o *overlay) Unpack(tag, name, layerType string, buildOnly bool) error {
	cacheDir := path.Join(o.config.StackerDir, "layer-bases", "oci")
	oci, err := umoci.OpenLayout(cacheDir)
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
					"--oci-path", cacheDir,
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

	err = ovl.mount(o.config, name)
	if err != nil {
		return err
	}

	if buildOnly {
		return nil
	}

	return lib.ImageCopy(lib.ImageCopyOpts{
		Src:  fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		Dest: fmt.Sprintf("oci:%s:%s", o.config.OCIDir, name),
	})
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

func generateLayer(config types.StackerConfig, mutator *mutate.Mutator, name string) (bool, error) {
	dir := path.Join(config.RootFSDir, name, "overlay")
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

	desc, err := mutator.Add(context.Background(), uncompressed, history)
	if err != nil {
		return false, err
	}

	// we're going to update the manifest at the end with these generated
	// layers, so we need to "extract" them. but there's no need to
	// actually extract them, we can just rename the contents we already
	// have for the generated hash, since that's where it came from.
	target := overlayPath(config, desc.Digest)
	err = os.MkdirAll(target, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't make new layer overlay dir")
	}

	err = os.Rename(dir, path.Join(target, "overlay"))
	if err != nil {
		return false, errors.Wrapf(err, "couldn't move overlay data to new location")
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't re-make overlay contents for %s", dir)
	}

	ovl := overlayMetadata{}
	ovl.Manifest, err = mutator.Manifest(context.Background())
	if err != nil {
		return false, err
	}

	// unmount the old overlay
	err = unix.Unmount(path.Join(config.RootFSDir, name, "rootfs"), 0)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't unmount old overlay")
	}

	err = ovl.write(config, name)
	if err != nil {
		return false, err
	}

	return true, ovl.mount(config, name)
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
		didMutate, err := generateLayer(config, mutator, buildLayer)
		if err != nil {
			return err
		}
		if didMutate {
			mutated = true
		}
	}

	didMutate, err := generateLayer(config, mutator, name)
	if err != nil {
		return err
	}

	// if we didn't do anything, don't do anything :)
	if !didMutate && !mutated {
		return nil
	}

	// for the actual layer, we need to update the descriptor in the output
	// too.
	newPath, err := mutator.Commit(context.Background())
	if err != nil {
		return err
	}

	return oci.UpdateReference(context.Background(), name, newPath.Root())
}
