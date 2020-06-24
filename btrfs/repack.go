package btrfs

import (
	"path"

	"github.com/anuvu/stacker/container"
)

func (b *btrfs) Repack(ociDir, name, layerType string) error {
	// ugh, need to thread "debug" through here.
	return container.RunUmociSubcommand(b.c, false, []string{
		"--oci-path", ociDir,
		"--tag", name,
		"--bundle-path", path.Join(b.c.RootFSDir, name),
		"repack",
		"--layer-type", layerType,
	})
}
