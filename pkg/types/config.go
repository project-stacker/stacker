package types

import (
	"embed"
	"fmt"
	"path"
)

// StackerConfig is a struct that contains global (or widely used) stacker
// config options.
type StackerConfig struct {
	StackerDir  string `yaml:"stacker_dir"`
	OCIDir      string `yaml:"oci_dir"`
	RootFSDir   string `yaml:"rootfs_dir"`
	Debug       bool   `yaml:"-"`
	StorageType string `yaml:"-"`

	// EmbeddedFS should contain a (statically linked) lxc-wrapper binary
	// (built from cmd/lxc-wrapper/lxc-wrapper.c) at
	// lxc-wrapper/lxc-wrapper.
	EmbeddedFS embed.FS `yaml:"-"`
}

// Substitutions - return an array of substitutions for StackerFiles
func (sc *StackerConfig) Substitutions() []string {
	return []string{
		fmt.Sprintf("STACKER_ROOTFS_DIR=%s", sc.RootFSDir),
		fmt.Sprintf("STACKER_STACKER_DIR=%s", sc.StackerDir),
		fmt.Sprintf("STACKER_OCI_DIR=%s", sc.OCIDir),
	}
}

func (sc *StackerConfig) CacheFile() string {
	return path.Join(sc.StackerDir, "build.cache")
}
