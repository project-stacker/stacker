package mount

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

const hasBtrfs = `nodev	sysfs
nodev	tmpfs
nodev	bdev
nodev	proc
nodev	cgroup
nodev	cgroup2
nodev	cpuset
nodev	devtmpfs
nodev	configfs
nodev	debugfs
nodev	tracefs
nodev	securityfs
nodev	sockfs
nodev	bpf
nodev	pipefs
nodev	ramfs
nodev	hugetlbfs
nodev	devpts
	ext3
	ext2
	ext4
	squashfs
	vfat
nodev	ecryptfs
	fuseblk
nodev	fuse
nodev	fusectl
nodev	efivarfs
nodev	mqueue
nodev	pstore
	btrfs
nodev	autofs
nodev	overlay
`

func TestBtrfsPresent(t *testing.T) {
	assert := assert.New(t)

	foundBtrfs, err := filesystemIsSupported("btrfs", bytes.NewReader([]byte(hasBtrfs)))
	assert.NoError(err)
	assert.True(foundBtrfs)
}

const noBtrfs = `nodev	sysfs
nodev	tmpfs
nodev	bdev
nodev	proc
nodev	cgroup
nodev	cgroup2
nodev	cpuset
nodev	devtmpfs
nodev	configfs
nodev	debugfs
nodev	tracefs
nodev	securityfs
nodev	sockfs
nodev	bpf
nodev	pipefs
nodev	ramfs
nodev	hugetlbfs
nodev	devpts
	ext3
	ext2
	ext4
	squashfs
	vfat
nodev	ecryptfs
	fuseblk
nodev	fuse
nodev	fusectl
nodev	efivarfs
nodev	mqueue
nodev	pstore
nodev	autofs
nodev	overlay
`

func TestBtrfsMissing(t *testing.T) {
	assert := assert.New(t)

	foundBtrfs, err := filesystemIsSupported("btrfs", bytes.NewReader([]byte(noBtrfs)))
	assert.NoError(err)
	assert.False(foundBtrfs)
}
