package stacker

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/openSUSE/umoci"
)

type BaseLayerOpts struct {
	Config StackerConfig
	Name   string
	Target string
	Layer  *Layer
	Cache  *BuildCache
	OCI    *umoci.Layout
}

func GetBaseLayer(o BaseLayerOpts, sf *Stackerfile) error {
	// delete the tag if it exists
	o.OCI.DeleteTag(o.Name)

	switch o.Layer.From.Type {
	case BuiltType:
		return getBuilt(o, sf)
	case TarType:
		return getTar(o)
	case OCIType:
		return getOCI(o)
	case DockerType:
		return getDocker(o)
	case ScratchType:
		return getScratch(o)
	default:
		return fmt.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

func tagFromSkopeoUrl(thing string) (string, error) {
	if strings.HasPrefix(thing, "docker") {
		url, err := url.Parse(thing)
		if err != nil {
			return "", err
		}

		if url.Path != "" {
			return path.Base(strings.Split(url.Path, ":")[0]), nil
		}

		// skopeo allows docker://centos:latest or
		// docker://docker.io/centos:latest; if we don't have a
		// url path, let's use the host as the image tag
		return strings.Split(url.Host, ":")[0], nil
	} else if strings.HasPrefix(thing, "oci") {
		pieces := strings.Split(thing, ":")
		if len(pieces) != 3 {
			return "", fmt.Errorf("bad OCI tag: %s", thing)
		}

		return pieces[2], nil
	} else {
		return "", fmt.Errorf("invalid image url: %s", thing)
	}
}

func runSkopeo(toImport string, o BaseLayerOpts, copyToOutput bool) error {
	tag, err := tagFromSkopeoUrl(toImport)
	if err != nil {
		return err
	}

	// Note that we can do tihs over the top of the cache every time, since
	// skopeo should be smart enough to only copy layers that have changed.
	cacheDir := path.Join(o.Config.StackerDir, "layer-bases", "oci")
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

		oci.GC()
	}()

	skopeoArgs := []string{
		// So we don't have to make everyone install an
		// /etc/containers/policy.json too. Alternatively, we could
		// write a default policy out to /tmp and use --policy.
		"--insecure-policy",
		"copy",
	}

	if o.Layer.From.Insecure {
		skopeoArgs = append(skopeoArgs, "--src-tls-verify=false")
	}

	skopeoArgs = append(skopeoArgs, toImport, fmt.Sprintf("oci:%s:%s", cacheDir, tag))

	cmd := exec.Command("skopeo", skopeoArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("skopeo copy: %s", err)
	}

	if !copyToOutput {
		return nil
	}

	// We just copied it to the cache, now let's copy that over to our image.
	cmd = exec.Command(
		"skopeo",
		"--insecure-policy",
		"copy",
		fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		fmt.Sprintf("oci:%s:%s", o.Config.OCIDir, tag),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo copy from cache to ocidir: %s: %s", err, string(output))
	}

	return nil
}

func extractOutput(o BaseLayerOpts) error {
	tag, err := o.Layer.From.ParseTag()
	if err != nil {
		return err
	}

	target := path.Join(o.Config.RootFSDir, o.Target)
	fmt.Println("unpacking to", target)

	image := fmt.Sprintf("%s:%s", path.Join(o.Config.StackerDir, "layer-bases", "oci"), tag)
	args := []string{"umoci", "unpack", "--image", image, target}
	err = MaybeRunInUserns(args, "image unpack failed")
	if err != nil {
		return err
	}

	// Delete the tag for the base layer; we're only interested in our
	// build layer outputs, not in the base layers.
	err = o.OCI.DeleteTag(tag)
	if err != nil {
		return err
	}

	return nil
}

func getDocker(o BaseLayerOpts) error {
	err := runSkopeo(o.Layer.From.Url, o, !o.Layer.BuildOnly)
	if err != nil {
		return err
	}

	return extractOutput(o)
}

func umociInit(o BaseLayerOpts) error {
	cmd := exec.Command(
		"umoci",
		"new",
		"--image",
		fmt.Sprintf("%s:%s", o.Config.OCIDir, o.Name))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umoci layout creation failed: %s: %s", err, string(output))
	}

	// N.B. This unpack doesn't need to be in a userns because it doesn't
	// actually do anything: the image is empty, and so it only makes the
	// rootfs dir and metadata files, which are owned by this user anyway.
	cmd = exec.Command(
		"umoci",
		"unpack",
		"--image",
		fmt.Sprintf("%s:%s", o.Config.OCIDir, o.Name),
		path.Join(o.Config.RootFSDir, ".working"))
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umoci empty unpack failed: %s: %s", err, string(output))
	}

	layerPath := path.Join(o.Config.RootFSDir, o.Target, "rootfs")
	if err := os.MkdirAll(layerPath, 0755); err != nil {
		return err
	}

	return nil
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

func getOCI(o BaseLayerOpts) error {
	err := runSkopeo(fmt.Sprintf("oci:%s", o.Layer.From.Url), o, !o.Layer.BuildOnly)
	if err != nil {
		return err
	}

	return extractOutput(o)
}

func getBuilt(o BaseLayerOpts, sf *Stackerfile) error {
	// We need to copy any base OCI layers to the output dir, since they
	// may not have been copied before and the final `umoci repack` expects
	// them to be there.
	base := o.Layer
	for {
		var ok bool
		base, ok = sf.Get(base.From.Tag)
		if !ok {
			return fmt.Errorf("missing base layer %s?", o.Layer.From.Tag)
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
	cmd := exec.Command(
		"skopeo",
		"--insecure-policy",
		"copy",
		fmt.Sprintf("oci:%s:%s", cacheDir, tag),
		fmt.Sprintf("oci:%s:%s", o.Config.OCIDir, tag),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo copy from cache to ocidir: %s: %s", err, string(output))
	}

	return o.OCI.DeleteTag(tag)
}
