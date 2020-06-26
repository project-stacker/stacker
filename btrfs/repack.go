package btrfs

import (
	"path"

	"github.com/anuvu/stacker/container"
)

func (b *btrfs) Repack(ociDir, name, layerType string) error {
	return container.RunUmociSubcommand(b.c, []string{
		"--oci-path", ociDir,
		"--tag", name,
		"--bundle-path", path.Join(b.c.RootFSDir, name),
		"repack",
		"--layer-type", layerType,
	})
}
