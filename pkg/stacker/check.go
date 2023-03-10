package stacker

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func KernelInfo() (string, error) {
	utsname := unix.Utsname{}
	if err := unix.Uname(&utsname); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s %s %s", string(utsname.Sysname[:]), string(utsname.Release[:]), string(utsname.Version[:])), nil
}

func MountInfo(path string) (string, error) {
	// from /usr/include/linux/magic.h
	var fstypeMap = map[int64]string{
		0xadf5:     "ADFS",
		0xadff:     "AFFS",
		0x5346414F: "AFS",
		0x0187:     "AUTOFS",
		0x73757245: "CODA",
		0x28cd3d45: "CRAMFS",
		0x453dcd28: "CRAMFS_WEND",
		0x64626720: "DEBUGFS",
		0x73636673: "SECURITYFS",
		0xf97cff8c: "SELINUX",
		0x43415d53: "SMACK",
		0x858458f6: "RAMFS",
		0x01021994: "TMPFS",
		0x958458f6: "HUGETLBFS",
		0x73717368: "SQUASHFS",
		0xf15f:     "ECRYPTFS",
		0x414A53:   "EFS",
		0xE0F5E1E2: "EROFS_V1",
		0xEF53:     "EXT2",
		0xabba1974: "XENFS",
		0x9123683E: "BTRFS",
		0x3434:     "NILFS",
		0xF2F52010: "F2FS",
		0xf995e849: "HPFS",
		0x9660:     "ISOFS",
		0x72b6:     "JFFS2",
		0x58465342: "XFS",
		0x6165676C: "PSTOREFS",
		0xde5e81e4: "EFIVARFS",
		0x00c0ffee: "HOSTFS",
		0x794c7630: "OVERLAYFS",
		0x137F:     "MINIX",
		0x138F:     "MINIX2",
		0x2468:     "MINIX2",
		0x2478:     "MINIX22",
		0x4d5a:     "MINIX3",
		0x4d44:     "MSDOS",
		0x564c:     "NCP",
		0x6969:     "NFS",
		0x7461636f: "OCFS2",
		0x9fa1:     "OPENPROM",
		0x002f:     "QNX4",
		0x68191122: "QNX6",
		0x6B414653: "AFS_FS",
		0x52654973: "REISERFS",
		0x517B:     "SMB",
		0x27e0eb:   "CGROUP",
		0x63677270: "CGROUP2",
		0x7655821:  "RDTGROUP",
		0x57AC6E9D: "STACK_END",
		0x74726163: "TRACEFS",
		0x01021997: "V9FS",
		0x62646576: "BDEVFS",
		0x64646178: "DAXFS",
		0x42494e4d: "BINFMTFS",
		0x1cd1:     "DEVPTS",
		0x6c6f6f70: "BINDERFS",
		0xBAD1DEA:  "FUTEXFS",
		0x50495045: "PIPEFS",
		0x9fa0:     "PROC",
		0x534F434B: "SOCKFS",
		0x62656572: "SYSFS",
		0x9fa2:     "USBDEVICE",
		0x11307854: "MTD_INODE_FS",
		0x09041934: "ANON_INODE_FS",
		0x73727279: "BTRFS_TEST",
		0x6e736673: "NSFS",
		0xcafe4a11: "BPF_FS",
		0x5a3c69f0: "AAFS",
		0x5a4f4653: "ZONEFS",
		0x15013346: "UDF",
		0x13661366: "BALLOON_KVM",
		0x58295829: "ZSMALLOC",
		0x444d4142: "DMA_BUF",
		0x454d444d: "DEVMEM",
		0x33:       "Z3FOLD",
		0xc7571590: "PPC_CMM",
		0x5345434d: "SECRETMEM",
		0x6a656a62: "SHIFTFS",
	}

	st := unix.Statfs_t{}
	if err := unix.Statfs(path, &st); err != nil {
		return "", err
	}

	fstype, ok := fstypeMap[st.Type]
	if !ok {
		fstype = "unknown"
	}

	// lookup fs type in /usr/include/linux/magic.h
	return fmt.Sprintf("%s(%x)", fstype, st.Type), nil
}
