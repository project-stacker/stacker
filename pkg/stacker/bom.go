package stacker

import (
	"fmt"
	"io"
	"os"
	"path"

	"stackerbuild.io/stacker/pkg/container"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

// build for all pkgs and then merge
func BuildLayerArtifacts(sc types.StackerConfig, storage types.Storage, l types.Layer,
	tag string, pkg types.Package,
) error {
	name, cleanup, err := storage.TemporaryWritableSnapshot(tag)
	if err != nil {
		return err
	}
	defer cleanup()

	c, err := container.New(sc, name)
	if err != nil {
		return err
	}
	defer c.Close()

	err = SetupBuildContainerConfig(sc, storage, c, tag)
	if err != nil {
		log.Errorf("build container %v", err)
		return err
	}

	err = SetupLayerConfig(sc, c, l, tag)
	if err != nil {
		return err
	}

	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}

	if err := c.BindMount(binary, "/static-stacker", ""); err != nil {
		return err
	}

	cmd := fmt.Sprintf("/static-stacker --oci-dir %s --roots-dir %s --stacker-dir %s --storage-type %s --internal-userns",
		sc.OCIDir, sc.RootFSDir, sc.StackerDir, sc.StorageType)

	if sc.Debug {
		cmd += " --debug"
	}

	cmd += " internal-go"

	author := l.Annotations[types.AuthorAnnotation]
	org := l.Annotations[types.OrgAnnotation]
	license := l.Annotations[types.LicenseAnnotation]
	dest := "/stacker-artifacts"
	cmd += fmt.Sprintf(" bom-build %s %s %s %s %s %s", dest, author, org, license, pkg.Name, pkg.Version)
	for _, ppath := range pkg.Paths {
		cmd += " " + ppath
	}
	err = c.Execute(cmd, os.Stdin)
	if err != nil {
		return err
	}

	return nil
}

func VerifyLayerArtifacts(sc types.StackerConfig, storage types.Storage, l types.Layer, tag string) error {
	name, cleanup, err := storage.TemporaryWritableSnapshot(tag)
	if err != nil {
		return err
	}
	defer cleanup()

	c, err := container.New(sc, name)
	if err != nil {
		return err
	}
	defer c.Close()

	err = SetupBuildContainerConfig(sc, storage, c, tag)
	if err != nil {
		log.Errorf("build container %v", err)
		return err
	}

	err = SetupLayerConfig(sc, c, l, tag)
	if err != nil {
		return err
	}

	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}

	if err := c.BindMount(binary, "/static-stacker", ""); err != nil {
		return err
	}

	cmd := fmt.Sprintf("/static-stacker --oci-dir %s --roots-dir %s --stacker-dir %s --storage-type %s --internal-userns",
		sc.OCIDir, sc.RootFSDir, sc.StackerDir, sc.StorageType)

	if sc.Debug {
		cmd += " --debug"
	}

	cmd += " internal-go"

	author := l.Annotations[types.AuthorAnnotation]
	org := l.Annotations[types.OrgAnnotation]

	dest := fmt.Sprintf("/stacker-artifacts/%s.json", tag)
	cmd += fmt.Sprintf(" bom-verify %s %s %s %s", dest, tag, author, org)
	err = c.Execute(cmd, os.Stdin)
	if err != nil {
		return err
	}

	return nil
}

func ImportArtifacts(sc types.StackerConfig, src types.ImageSource, name string) error {
	if src.Type == types.BuiltLayer {
		// if a bom is available, add it here so it can be merged
		srcpath := path.Join(sc.StackerDir, "artifacts", src.Tag, fmt.Sprintf("%s.json", src.Tag))

		dstfp, err := os.CreateTemp(path.Join(sc.StackerDir, "artifacts", name), fmt.Sprintf("%s-*.json", name))
		if err != nil {
			return err
		}
		defer dstfp.Close()

		srcfp, err := os.Open(srcpath)
		if err != nil {
			return err
		}
		defer srcfp.Close()

		if _, err := io.Copy(dstfp, srcfp); err != nil {
			return err
		}
	}

	return nil
}
