package lib

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	stackeroci "github.com/project-stacker/stacker/oci"
	"github.com/project-stacker/stacker/squashfs"
	"github.com/stretchr/testify/assert"
)

func createImage(dir string, tag string) error {
	imageRoot := path.Join(dir, "oci")

	var oci casext.Engine
	_, err := os.Stat(imageRoot)
	if err != nil {
		oci, err = umoci.CreateLayout(imageRoot)
	} else {
		oci, err = umoci.OpenLayout(imageRoot)
	}
	if err != nil {
		return err
	}
	defer oci.Close()

	err = umoci.NewImage(oci, tag)
	if err != nil {
		return err
	}

	// need *something* in the layer, why not just recursively include the
	// OCI image for maximum confusion :)
	layer, err := squashfs.MakeSquashfs(dir, path.Join(dir, "oci"), nil)
	if err != nil {
		return err
	}

	_, err = stackeroci.AddBlobNoCompression(oci, tag, layer)
	if err != nil {
		return err
	}

	return oci.GC(context.Background())
}

func TestImageCompressionCopy(t *testing.T) {
	assert := assert.New(t)
	dir, err := ioutil.TempDir("", "stacker-compression-copy-test")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	assert.NoError(createImage(dir, "foo"))

	assert.NoError(ImageCopy(ImageCopyOpts{
		Src:  fmt.Sprintf("oci:%s/oci:foo", dir),
		Dest: fmt.Sprintf("oci:%s/oci2:foo", dir),
	}))

	origBlobs, err := ioutil.ReadDir(fmt.Sprintf("%s/oci/blobs/sha256/", dir))
	assert.NoError(err)
	copiedBlobs, err := ioutil.ReadDir(fmt.Sprintf("%s/oci2/blobs/sha256/", dir))
	assert.NoError(err)

	for i := range origBlobs {
		// could check the hashes too, but containers/image doesn't
		// generally break that :)
		assert.Equal(origBlobs[i].Name(), copiedBlobs[i].Name())
	}
}

func TestForceManifestTypeOption(t *testing.T) {
	assert := assert.New(t)
	dir, err := ioutil.TempDir("", "stacker-force-manifesttype-test")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	assert.NoError(createImage(dir, "foo"))

	assert.NoError(ImageCopy(ImageCopyOpts{
		Src:               fmt.Sprintf("oci:%s/oci:foo", dir),
		Dest:              fmt.Sprintf("oci:%s/oci2:foo", dir),
		ForceManifestType: ispec.MediaTypeImageManifest,
	}))

	assert.Error(ImageCopy(ImageCopyOpts{
		Src:               fmt.Sprintf("oci:%s/oci:foo", dir),
		Dest:              fmt.Sprintf("oci:%s/oci2:foo", dir),
		ForceManifestType: "test",
	}))
}

func TestOldManifestReallyRemoved(t *testing.T) {
	assert := assert.New(t)
	dir, err := ioutil.TempDir("", "stacker-compression-copy-test")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	assert.NoError(createImage(dir, "a"))
	assert.NoError(createImage(dir, "b"))

	assert.NoError(ImageCopy(ImageCopyOpts{
		Src:  fmt.Sprintf("oci:%s/oci:a", dir),
		Dest: fmt.Sprintf("oci:%s/oci2:tag", dir),
	}))
	assert.NoError(ImageCopy(ImageCopyOpts{
		Src:  fmt.Sprintf("oci:%s/oci:b", dir),
		Dest: fmt.Sprintf("oci:%s/oci2:tag", dir),
	}))

	oci, err := umoci.OpenLayout(path.Join(dir, "oci2"))
	assert.NoError(err)
	defer oci.Close()

	ctx := context.Background()

	index, err := oci.GetIndex(ctx)
	assert.NoError(err)
	assert.Len(index.Manifests, 1)
}
