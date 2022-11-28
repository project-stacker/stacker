package stacker

import (
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
	"stackerbuild.io/stacker/container"
	"stackerbuild.io/stacker/types"
)

func Grab(sc types.StackerConfig, storage types.Storage, name string, source string, targetDir string) error {
	c, err := container.New(sc, name)
	if err != nil {
		return err
	}
	defer c.Close()

	err = c.BindMount(targetDir, "/stacker", "")
	if err != nil {
		return err
	}
	defer os.Remove(path.Join(sc.RootFSDir, name, "rootfs", "stacker"))

	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return errors.Wrapf(err, "couldn't find executable for bind mount")
	}

	err = c.BindMount(binary, "/static-stacker", "")
	if err != nil {
		return err
	}

	err = SetupBuildContainerConfig(sc, storage, c, name)
	if err != nil {
		return err
	}

	return c.Execute(fmt.Sprintf("/static-stacker internal-go cp %s /stacker/%s", source, path.Base(source)), nil)
}
