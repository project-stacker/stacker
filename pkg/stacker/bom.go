package stacker

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

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

	inDir := types.InternalStackerDir
	err = SetupBuildContainerConfig(sc, storage, c, inDir, name)
	if err != nil {
		log.Errorf("build container %v", err)
		return err
	}

	err = SetupLayerConfig(sc, c, l, inDir, tag)
	if err != nil {
		return err
	}

	cmd := []string{filepath.Join(inDir, types.BinStacker)}

	if sc.Debug {
		cmd = append(cmd, "--debug")
	}

	cmd = append(cmd, "bom", "build", filepath.Join(inDir, "artifacts"),
		l.Annotations[types.AuthorAnnotation],
		l.Annotations[types.OrgAnnotation],
		l.Annotations[types.LicenseAnnotation],
		pkg.Name, pkg.Version)
	cmd = append(cmd, pkg.Paths...)
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

	inDir := types.InternalStackerDir
	err = SetupBuildContainerConfig(sc, storage, c, inDir, name)
	if err != nil {
		log.Errorf("build container %v", err)
		return err
	}

	err = SetupLayerConfig(sc, c, l, inDir, tag)
	if err != nil {
		return err
	}

	cmd := []string{filepath.Join(inDir, types.BinStacker)}

	if sc.Debug {
		cmd = append(cmd, "--debug")
	}

	cmd = append(cmd, "bom", "verify",
		fmt.Sprintf(types.InternalStackerDir+"/artifacts/%s.json", tag),
		tag, l.Annotations[types.AuthorAnnotation], l.Annotations[types.OrgAnnotation])

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
