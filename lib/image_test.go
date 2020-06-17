package lib

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	stackeroci "github.com/anuvu/stacker/oci"
	"github.com/anuvu/stacker/squashfs"
	"github.com/openSUSE/umoci"
	"github.com/stretchr/testify/assert"
)

func createImage(dir string) error {
	oci, err := umoci.CreateLayout(path.Join(dir, "oci"))
	if err != nil {
		return err
	}
	defer oci.Close()

	err = umoci.NewImage(oci, "foo")
	if err != nil {
		return err
	}

	// need *something* in the layer, why not just recursively include the
	// OCI image for maximum confusion :)
	layer, err := squashfs.MakeSquashfs(dir, path.Join(dir, "oci"), nil)
	if err != nil {
		return err
	}

	_, err = stackeroci.AddBlobNoCompression(oci, "foo", layer)
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

	assert.NoError(createImage(dir))

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
