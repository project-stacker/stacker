package erofs

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/pkg/errors"
)

const (
	// Definitions for superblock.
	superblockMagicV1 = 0xe0f5e1e2
	superblockMagic   = superblockMagicV1
	SuperBlockOffset  = 1024

	// Inode slot size in bit shift.
	InodeSlotBits = 5

	// Max file name length.
	MaxNameLen = 255
)

// Bit definitions for Inode*::Format.
const (
	InodeLayoutBit  = 0
	InodeLayoutBits = 1

	InodeDataLayoutBit  = 1
	InodeDataLayoutBits = 3
)

// Inode layouts.
const (
	InodeLayoutCompact  = 0
	InodeLayoutExtended = 1
)

// Inode data layouts.
const (
	InodeDataLayoutFlatPlain = iota
	InodeDataLayoutFlatCompressionLegacy
	InodeDataLayoutFlatInline
	InodeDataLayoutFlatCompression
	InodeDataLayoutChunkBased
	InodeDataLayoutMax
)

// Features w/ backward compatibility.
// This is not exhaustive, unused features are not listed.
const (
	FeatureCompatSuperBlockChecksum = 0x00000001
)

// Features w/o backward compatibility.
//
// Any features that aren't in FeatureIncompatSupported are incompatible
// with this implementation.
//
// This is not exhaustive, unused features are not listed.
const (
	FeatureIncompatSupported = 0x0
)

// Sizes of on-disk structures in bytes.
const (
	superblockSize    = 128
	InodeCompactSize  = 32
	InodeExtendedSize = 64
	DirentSize        = 12
)

type superblock struct {
	Magic           uint32
	Checksum        uint32
	FeatureCompat   uint32
	BlockSizeBits   uint8
	ExtSlots        uint8
	RootNid         uint16
	Inodes          uint64
	BuildTime       uint64
	BuildTimeNsec   uint32
	Blocks          uint32
	MetaBlockAddr   uint32
	XattrBlockAddr  uint32
	UUID            [16]uint8
	VolumeName      [16]uint8
	FeatureIncompat uint32
	Union1          uint16
	ExtraDevices    uint16
	DevTableSlotOff uint16
	Reserved        [38]uint8
}

/*
// checkRange checks whether the range [off, off+n) is valid.
func (i *Image) checkRange(off, n uint64) bool {
	size := uint64(len(i.bytes))
	end := off + n
	return off < size && off <= end && end <= size
}

// BytesAt returns the bytes at [off, off+n) of the image.
func (i *Image) BytesAt(off, n uint64) ([]byte, error) {
	if ok := i.checkRange(off, n); !ok {
		//log.Warningf("Invalid byte range (off: 0x%x, n: 0x%x) for image (size: 0x%x)", off, n, len(i.bytes))
		return nil, linuxerr.EFAULT
	}
	return i.bytes[off : off+n], nil
}

// unmarshalAt deserializes data from the bytes at [off, off+n) of the image.
func (i *Image) unmarshalAt(data marshal.Marshallable, off uint64) error {
	bytes, err := i.BytesAt(off, uint64(data.SizeBytes()))
	if err != nil {
		//log.Warningf("Failed to deserialize %T from 0x%x.", data, off)
		return err
	}
	data.UnmarshalUnsafe(bytes)
	return nil
}

// initSuperBlock initializes the superblock of this image.
func (i *Image) initSuperBlock() error {
	// i.sb is used in the hot path. Let's save a copy of the superblock.
	if err := i.unmarshalAt(&i.sb, SuperBlockOffset); err != nil {
		return fmt.Errorf("image size is too small")
	}

	if i.sb.Magic != SuperBlockMagicV1 {
		return fmt.Errorf("unknown magic: 0x%x", i.sb.Magic)
	}

	if err := i.verifyChecksum(); err != nil {
		return err
	}

	if featureIncompat := i.sb.FeatureIncompat & ^uint32(FeatureIncompatSupported); featureIncompat != 0 {
		return fmt.Errorf("unsupported incompatible features detected: 0x%x", featureIncompat)
	}

	if i.BlockSize()%hostarch.PageSize != 0 {
		return fmt.Errorf("unsupported block size: 0x%x", i.BlockSize())
	}

	return nil
}

// verifyChecksum verifies the checksum of the superblock.
func (i *Image) verifyChecksum() error {
	if i.sb.FeatureCompat&FeatureCompatSuperBlockChecksum == 0 {
		return nil
	}

	sb := i.sb
	sb.Checksum = 0
	table := crc32.MakeTable(crc32.Castagnoli)
	checksum := crc32.Checksum(marshal.Marshal(&sb), table)
// unmarshalAt deserializes data from the bytes at [off, off+n) of the image.
func (i *Image) unmarshalAt(data marshal.Marshallable, off uint64) error {
	bytes, err := i.BytesAt(off, uint64(data.SizeBytes()))
	if err != nil {
		log.Warningf("Failed to deserialize %T from 0x%x.", data, off)
		return err
	}
	data.UnmarshalUnsafe(bytes)
	return nil
}
	off := SuperBlockOffset + uint64(i.sb.SizeBytes())
	if bytes, err := i.BytesAt(off, uint64(i.BlockSize())-off); err != nil {
		return fmt.Errorf("image size is too small")
	} else {
		checksum = ^crc32.Update(checksum, table, bytes)
	}
	if checksum != i.sb.Checksum {
		return fmt.Errorf("invalid checksum: 0x%x, expected: 0x%x", checksum, i.sb.Checksum)
	}

	return nil
}
*/

func parseSuperblock(b []byte) (*superblock, error) {
	var s *superblock = &superblock{}

	if len(b) != superblockSize {
		return nil, errors.Errorf("superblock had %d bytes instead of expected %d", len(b), superblockSize)
	}

	magic := binary.LittleEndian.Uint32(b[0:4])
	if magic != superblockMagic {
		return nil, errors.Errorf("superblock had magic of %d instead of expected %d", magic, superblockMagic)
	}

	/*
		magic := binary.LittleEndian.Uint32(b[0:4])
		if magic != superblockMagic {
			return nil, errors.Errorf("superblock had magic of %d instead of expected %d", magic, superblockMagic)
		}
		majorVersion := binary.LittleEndian.Uint16(b[28:30])
		minorVersion := binary.LittleEndian.Uint16(b[30:32])
		if majorVersion != superblockMajorVersion || minorVersion != superblockMinorVersion {
			return nil, errors.Errorf("superblock version mismatch, received %d.%d instead of expected %d.%d", majorVersion, minorVersion, superblockMajorVersion, superblockMinorVersion)
		}

		blocksize := binary.LittleEndian.Uint32(b[12:16])
		blocklog := binary.LittleEndian.Uint16(b[22:24])
		expectedLog := uint16(math.Log2(float64(blocksize)))
		if expectedLog != blocklog {
			return nil, errors.Errorf("superblock block log mismatch, actual %d expected %d", blocklog, expectedLog)
		}
		flags, err := parseFlags(b[24:26])
		if err != nil {
			return nil, errors.Errorf("error parsing flags bytes: %v", err)
		}
		s := &superblock{
			inodes:              binary.LittleEndian.Uint32(b[4:8]),
			modTime:             time.Unix(int64(binary.LittleEndian.Uint32(b[8:12])), 0),
			blocksize:           blocksize,
			fragmentCount:       binary.LittleEndian.Uint32(b[16:20]),
			compression:         compression(binary.LittleEndian.Uint16(b[20:22])),
			idCount:             binary.LittleEndian.Uint16(b[26:28]),
			versionMajor:        binary.LittleEndian.Uint16(b[28:30]),
			versionMinor:        binary.LittleEndian.Uint16(b[30:32]),
			rootInode:           parseRootInode(binary.LittleEndian.Uint64(b[32:40])),
			size:                binary.LittleEndian.Uint64(b[40:48]),
			idTableStart:        binary.LittleEndian.Uint64(b[48:56]),
			xattrTableStart:     binary.LittleEndian.Uint64(b[56:64]),
			inodeTableStart:     binary.LittleEndian.Uint64(b[64:72]),
			directoryTableStart: binary.LittleEndian.Uint64(b[72:80]),
			fragmentTableStart:  binary.LittleEndian.Uint64(b[80:88]),
			exportTableStart:    binary.LittleEndian.Uint64(b[88:96]),
			superblockFlags:     *flags,
		}
	*/
	return s, nil
}

func readSuperblock(path string) (*superblock, error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	buf := make([]byte, superblockSize)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return nil, err
	}

	return parseSuperblock(buf)
}
