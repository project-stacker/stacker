package squashfs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerityMetadata(t *testing.T) {
	assert := assert.New(t)

	rootfs, err := os.MkdirTemp("", "stacker_verity_test_rootfs")
	assert.NoError(err)
	defer os.RemoveAll(rootfs)

	tempdir, err := os.MkdirTemp("", "stacker_verity_test_tempdir")
	assert.NoError(err)
	defer os.RemoveAll(tempdir)

	err = os.WriteFile(path.Join(rootfs, "foo"), []byte("bar"), 0644)
	assert.NoError(err)

	reader, _, rootHash, err := MakeSquashfs(tempdir, rootfs, nil, VerityMetadataPresent)
	if err == cryptsetupTooOld {
		t.Skip("libcryptsetup too old")
	}
	assert.NoError(err)

	content, err := io.ReadAll(reader)
	assert.NoError(err)
	squashfsFile := path.Join(tempdir, "foo.squashfs")
	err = os.WriteFile(squashfsFile, content, 0600)
	assert.NoError(err)

	verityOffset, err := verityDataLocation(squashfsFile)
	assert.NoError(err)

	// now let's try to verify it at least in userspace. exec cryptsetup
	// because i'm lazy and it's only in tests
	cmd := exec.Command("veritysetup", "verify", squashfsFile, squashfsFile, rootHash,
		"--hash-offset", fmt.Sprintf("%d", verityOffset))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	assert.NoError(err)

	// what if we fiddle with the verity data? note that we have to fiddle
	// with the beginning of the verity block, which will be 4k long for
	// our small squashfs file, because the stuff at the end of the verity
	// block is unused.
	const bytesToFlip = 2
	const flipAtOffset = -4087

	f, err := os.OpenFile(squashfsFile, os.O_RDWR, 0644)
	assert.NoError(err)
	defer f.Close()
	_, err = f.Seek(flipAtOffset, os.SEEK_END)
	assert.NoError(err)

	buf := make([]byte, bytesToFlip)
	n, err := f.Read(buf)
	assert.Equal(n, bytesToFlip)
	assert.NoError(err)

	for i := range buf {
		buf[i] = buf[i] ^ 0xff
	}

	_, err = f.Seek(flipAtOffset, os.SEEK_END)
	assert.NoError(err)
	n, err = f.Write(buf)
	assert.Equal(n, bytesToFlip)
	assert.NoError(err)
	assert.NoError(f.Sync())
	assert.NoError(f.Close())

	cmd = exec.Command("veritysetup", "verify", squashfsFile, squashfsFile, rootHash,
		"--hash-offset", fmt.Sprintf("%d", verityOffset))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	assert.Error(err)
}
