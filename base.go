package stacker

import (
	"fmt"
	"os"
	"os/exec"
	"path"

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

func GetBaseLayer(o BaseLayerOpts) error {
	switch o.Layer.From.Type {
	case BuiltType:
		/* nothing to do assuming layers are imported in dependency order */
		return nil
	case TarType:
		return getTar(o)
	case OCIType:
		return fmt.Errorf("not implemented")
	case DockerType:
		return getDocker(o)
	case ScratchType:
		return getScratch(o)
	default:
		return fmt.Errorf("unknown layer type: %v", o.Layer.From.Type)
	}
}

func getDocker(o BaseLayerOpts) error {
	tag, err := o.Layer.From.ParseTag()
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

	skopeoArgs = append(skopeoArgs, o.Layer.From.Url, fmt.Sprintf("oci:%s:%s", cacheDir, tag))

	cmd := exec.Command("skopeo", skopeoArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("skopeo copy: %s", err)
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

	target := path.Join(o.Config.RootFSDir, o.Target)
	fmt.Println("unpacking to", target)

	image := fmt.Sprintf("%s:%s", o.Config.OCIDir, tag)
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
