package stacker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/anuvu/stacker/lib"
	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	"github.com/openSUSE/umoci/pkg/fseval"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/vbatts/go-mtree"
)

type BaseLayerOpts struct {
	Config    StackerConfig
	Name      string
	Target    string
	Layer     *Layer
	Cache     *BuildCache
	OCI       casext.Engine
	LayerType string
	Debug     bool
}

func GetBaseLayer(o BaseLayerOpts, sfm StackerFiles) error {
	// delete the tag if it exists
	o.OCI.DeleteReference(context.Background(), o.Name)

	switch o.Layer.From.Type {
	case BuiltType:
		return getBuilt(o, sfm)
	case TarType:
		return getTar(o)
	case OCIType:
		return getContainersImageType(o)
	case DockerType:
		return getContainersImageType(o)
	case ScratchType:
		return getScratch(o)
	default:
		return fmt.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

func importImage(is *ImageSource, config StackerConfig) error {
	toImport, err := is.ContainersImageURL()
	if err != nil {
		return err
	}

	tag, err := is.ParseTag()
	if err != nil {
		return err
	}

	// Note that we can do tihs over the top of the cache every time, since
	// skopeo should be smart enough to only copy layers that have changed.
	cacheDir := path.Join(config.StackerDir, "layer-bases", "oci")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	defer func() {
		oci, err := umoci.OpenLayout(cacheDir)
		if err != nil {
			// Some error might have occurred, in which case we
			// don't have a valid OCI layout, which is fine.
			return
		}
		defer oci.Close()
	}()

	fmt.Printf("loading %s\n", toImport)
	err = lib.ImageCopy(lib.ImageCopyOpts{
		Src:      toImport,
		Dest:     fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		SkipTLS:  is.Insecure,
		Progress: os.Stdout,
	})
	if err != nil {
		return err
	}

	return err
}

func extractOutput(o BaseLayerOpts) error {
	tag, err := o.Layer.From.ParseTag()
	if err != nil {
		return err
	}

	target := path.Join(o.Config.RootFSDir, o.Target)
	fmt.Println("unpacking to", target)

	cacheDir := path.Join(o.Config.StackerDir, "layer-bases", "oci")
	cacheTag, err := o.Layer.From.ParseTag()
	if err != nil {
		return err
	}

	cacheOCI, err := umoci.OpenLayout(cacheDir)
	if err != nil {
		return err
	}

	sourceLayerType := "tar"
	manifest, err := stackeroci.LookupManifest(cacheOCI, tag)
	if err != nil {
		return err
	}

	if manifest.Layers[0].MediaType == stackeroci.MediaTypeLayerSquashfs {
		sourceLayerType = "squashfs"
	}

	if sourceLayerType == "squashfs" {
		for _, layer := range manifest.Layers {
			rootfs := path.Join(target, "rootfs")
			squashfsFile := path.Join(cacheDir, "blobs", "sha256", layer.Digest.Encoded())
			err = MaybeRunInUserns([]string{"unsquashfs", "-f", "-d", rootfs, squashfsFile}, "couldn't unsquashfs layer")
			if err != nil {
				return err
			}
		}

		dps, err := cacheOCI.ResolveReference(context.Background(), tag)
		if err != nil {
			return err
		}

		mtreeName := strings.Replace(dps[0].Descriptor().Digest.String(), ":", "_", 1)
		err = umoci.GenerateBundleManifest(mtreeName, target, fseval.DefaultFsEval)
		if err != nil {
			return err
		}

		err = umoci.WriteBundleMeta(target, umoci.Meta{
			Version: umoci.MetaVersion,
			From: casext.DescriptorPath{
				Walk: []ispec.Descriptor{dps[0].Descriptor()},
			},
		})
	} else {
		// This is a bit of a hack; since we want to unpack from the
		// layer-bases import folder instead of the actual oci dir, we hack
		// this to make config.OCIDir be our input folder. That's a lie, but it
		// seems better to do a little lie here than to try and abstract it out
		// and make everyone else deal with it.
		modifiedConfig := o.Config
		modifiedConfig.OCIDir = cacheDir
		err = RunUmociSubcommand(modifiedConfig, o.Debug, []string{
			"--bundle-path", target,
			"--tag", tag,
			"unpack",
		})
		if err != nil {
			return err
		}
	}

	// Delete the tag for the base layer; we're only interested in our
	// build layer outputs, not in the base layers.
	o.OCI.DeleteReference(context.Background(), tag)

	if o.Layer.BuildOnly {
		return nil
	}

	// if the layer types are the same, just copy it over and be done
	if o.LayerType == sourceLayerType {
		// We just copied it to the cache, now let's copy that over to our image.
		err = lib.ImageCopy(lib.ImageCopyOpts{
			Src:  fmt.Sprintf("oci:%s:%s", cacheDir, tag),
			Dest: fmt.Sprintf("oci:%s:%s", o.Config.OCIDir, o.Name),
		})
		return err
	}

	var blob io.ReadCloser

	bundlePath := path.Join(o.Config.RootFSDir, WorkingContainerName)
	// otherwise, render the right layer type
	if o.LayerType == "squashfs" {
		// sourced a non-squashfs image and wants a squashfs layer,
		// let's generate one.
		o.OCI.GC(context.Background())

		tmpSquashfs, err := mkSquashfs(o.Config, nil)
		if err != nil {
			return err
		}

		blob = tmpSquashfs

	} else {
		// sourced a non-tar layer, and wants a tar one.
		diff, err := mtree.Check(path.Join(bundlePath, "rootfs"), nil, umoci.MtreeKeywords, fseval.DefaultFsEval)
		if err != nil {
			return err
		}

		blob, err = layer.GenerateLayer(path.Join(bundlePath, "rootfs"), diff, nil)
		if err != nil {
			return err
		}
	}

	layerDigest, layerSize, err := o.OCI.PutBlob(context.Background(), blob)
	if err != nil {
		return err
	}

	cacheManifest, err := stackeroci.LookupManifest(cacheOCI, cacheTag)
	if err != nil {
		return err
	}

	config, err := stackeroci.LookupConfig(cacheOCI, cacheManifest.Config)
	if err != nil {
		return err
	}

	layerType := stackeroci.MediaTypeLayerSquashfs
	if o.LayerType == "tar" {
		layerType = ispec.MediaTypeImageLayerGzip
	}

	desc := ispec.Descriptor{
		MediaType: layerType,
		Digest:    layerDigest,
		Size:      layerSize,
	}

	manifest.Layers = []ispec.Descriptor{desc}
	config.RootFS.DiffIDs = []digest.Digest{layerDigest}
	now := time.Now()
	config.History = []ispec.History{{
		Created:   &now,
		CreatedBy: fmt.Sprintf("stacker layer-type mismatch repack of %s", tag),
	},
	}

	configDigest, configSize, err := o.OCI.PutBlobJSON(context.Background(), config)
	if err != nil {
		return err
	}

	manifest.Config = ispec.Descriptor{
		MediaType: ispec.MediaTypeImageConfig,
		Digest:    configDigest,
		Size:      configSize,
	}

	manifestDigest, manifestSize, err := o.OCI.PutBlobJSON(context.Background(), manifest)
	if err != nil {
		return err
	}

	desc = ispec.Descriptor{
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}

	err = o.OCI.UpdateReference(context.Background(), o.Name, desc)
	if err != nil {
		return err
	}

	err = updateBundleMtree(bundlePath, desc)
	if err != nil {
		return err
	}

	err = umoci.WriteBundleMeta(bundlePath, umoci.Meta{
		Version: umoci.MetaVersion,
		From: casext.DescriptorPath{
			Walk: []ispec.Descriptor{desc},
		},
	})
	return err
}

func umociInit(o BaseLayerOpts) error {
	return RunUmociSubcommand(o.Config, o.Debug, []string{
		"--tag", o.Name,
		"--bundle-path", path.Join(o.Config.RootFSDir, WorkingContainerName),
		"init",
	})
}

func getTar(o BaseLayerOpts) error {
	cacheDir := path.Join(o.Config.StackerDir, "layer-bases")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	tar, err := acquireUrl(o.Config, o.Layer.From.Url, cacheDir)
	if err != nil {
		return err
	}

	err = umociInit(o)
	if err != nil {
		return err
	}

	// TODO: make this respect ID maps
	layerPath := path.Join(o.Config.RootFSDir, o.Target, "rootfs")
	output, err := exec.Command("tar", "xf", tar, "-C", layerPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %s: %s", err, string(output))
	}

	return nil
}

func getScratch(o BaseLayerOpts) error {
	return umociInit(o)
}

func getContainersImageType(o BaseLayerOpts) error {
	err := importImage(o.Layer.From, o.Config)
	if err != nil {
		return err
	}

	return extractOutput(o)
}

func getBuilt(o BaseLayerOpts, sfm StackerFiles) error {
	// We need to copy any base OCI layers to the output dir, since they
	// may not have been copied before and the final `umoci repack` expects
	// them to be there.
	base := o.Layer
	for {
		// Iterate through base layers until we find the first one which is not BuiltType
		// Need to declare ok separately, if we do it in the same line as
		// assigning the new value to base, base would be a new variable only in the scope
		// of this iteration and we never meet the condition to exit the loop
		var ok bool
		base, ok = sfm.LookupLayerDefinition(base.From.Tag)
		if !ok {
			return fmt.Errorf("missing base layer: %s?", base.From.Tag)
		}

		if base.From.Type != BuiltType {
			break
		}
	}

	// Nothing to do here -- we didn't import any base layers.
	if (base.From.Type != DockerType && base.From.Type != OCIType) || !base.BuildOnly {
		return nil
	}

	// Nothing to do here either -- the previous step emitted a layer with
	// the base's tag name. We don't want to overwrite that with a stock
	// base layer.
	if !base.BuildOnly {
		return nil
	}

	tag, err := base.From.ParseTag()
	if err != nil {
		return err
	}

	cacheDir := path.Join(o.Config.StackerDir, "layer-bases", "oci")
	err = lib.ImageCopy(lib.ImageCopyOpts{
		Src:  fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		Dest: fmt.Sprintf("oci:%s:%s", o.Config.OCIDir, tag),
	})
	if err != nil {
		return err
	}

	return o.OCI.DeleteReference(context.Background(), tag)
}

func ComputeAggregateHash(manifest ispec.Manifest, descriptor ispec.Descriptor) (string, error) {
	h := sha256.New()
	found := false

	for _, l := range manifest.Layers {
		_, err := h.Write([]byte(l.Digest.String()))
		if err != nil {
			return "", err
		}

		if l.Digest.String() == descriptor.Digest.String() {
			found = true
			break
		}
	}

	if !found {
		return "", errors.Errorf("couldn't find descriptor %s in manifest %s", descriptor.Digest.String(), manifest.Annotations["org.opencontainers.image.ref.name"])
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func RunUmociSubcommand(config StackerConfig, debug bool, args []string) error {
	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}

	cmd := []string{
		binary,
		"--oci-dir", config.OCIDir,
		"--roots-dir", config.RootFSDir,
		"--stacker-dir", config.StackerDir,
	}

	if debug {
		cmd = append(cmd, "--debug")
	}

	cmd = append(cmd, "umoci")
	cmd = append(cmd, args...)
	return MaybeRunInUserns(cmd, "image unpack failed")
}
