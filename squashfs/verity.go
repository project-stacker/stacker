package squashfs

// #cgo pkg-config: libcryptsetup libsquashfs1 --static
// #include <libcryptsetup.h>
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/anuvu/squashfs"
	"github.com/martinjungblut/go-cryptsetup"
	"github.com/pkg/errors"
)

const VerityRootHashAnnotation = "com.cisco.stacker.squashfs_verity_root_hash"

type verityDeviceType struct {
	Flags      uint
	DataDevice string
	HashOffset int
}

func (verity verityDeviceType) Name() string {
	return "VERITY"
}

func (verity verityDeviceType) Unmanaged() (unsafe.Pointer, func()) {
	var cParams C.struct_crypt_params_verity

	cParams.hash_name = C.CString("sha256")
	cParams.data_device = C.CString(verity.DataDevice)
	cParams.fec_device = nil
	cParams.fec_roots = 0

	cParams.salt_size = 32 // DEFAULT_VERITY_SALT_SIZE for x86
	cParams.salt = nil

	// these can't be larger than a page size, but we want them to be as
	// big as possible so the hash data is small, so let's set them to a
	// page size.
	cParams.data_block_size = C.uint(os.Getpagesize())
	cParams.hash_block_size = C.uint(os.Getpagesize())

	cParams.data_size = C.ulong(verity.HashOffset / os.Getpagesize())
	cParams.hash_area_offset = C.ulong(verity.HashOffset)
	cParams.fec_area_offset = 0
	cParams.hash_type = 1 // use format version 1 (i.e. "modern", non chrome-os)
	cParams.flags = C.uint(verity.Flags)

	deallocate := func() {
		C.free(unsafe.Pointer(cParams.hash_name))
		C.free(unsafe.Pointer(cParams.data_device))
	}

	return unsafe.Pointer(&cParams), deallocate
}

func isCryptsetupEINVAL(err error) bool {
	cse, ok := err.(*cryptsetup.Error)
	return ok && cse.Code() == -22
}

var cryptsetupTooOld = errors.Errorf("libcryptsetup not new enough, need >= 2.3.0")

func appendVerityData(file string) (string, error) {
	fi, err := os.Lstat(file)
	if err != nil {
		return "", errors.WithStack(err)
	}

	verityOffset := fi.Size()

	// we expect mksquashfs to have padded the file to the nearest 4k
	// (dm-verity requires device block size, which is 512 for loopback,
	// which is a multiple of 4k), let's check that here
	if verityOffset%512 != 0 {
		return "", errors.Errorf("bad verity file size %d", verityOffset)
	}

	verityDevice, err := cryptsetup.Init(file)
	if err != nil {
		return "", errors.WithStack(err)
	}

	verityType := verityDeviceType{
		Flags:      cryptsetup.CRYPT_VERITY_CREATE_HASH,
		DataDevice: file,
		HashOffset: int(verityOffset),
	}
	err = verityDevice.Format(verityType, cryptsetup.GenericParams{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	// a bit ugly, but this is the only API for querying the root
	// hash (short of invoking the veritysetup binary), and it was
	// added in libcryptsetup commit 188cb114af94 ("Add support for
	// verity in crypt_volume_key_get and use it in status"), which
	// is relatively recent (ubuntu 20.04 does not have this patch,
	// for example).
	//
	// before that, we get a -22. so, let's test for that and
	// render a special error message.
	rootHash, _, err := verityDevice.VolumeKeyGet(cryptsetup.CRYPT_ANY_SLOT, "")
	if isCryptsetupEINVAL(err) {
		return "", cryptsetupTooOld
	} else if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", rootHash), errors.WithStack(err)
}

func VerityDataLocation(file string) (uint64, error) {
	sqfs, err := squashfs.OpenSquashfs(file)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer sqfs.Close()
	squashLen := sqfs.BytesUsed()

	// squashfs is padded out to the nearest 4k
	if squashLen%4096 != 0 {
		squashLen = squashLen + (4096 - squashLen%4096)
	}

	return squashLen, nil
}
