package stacker

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/pkg/errors"
	"stackerbuild.io/stacker/pkg/container"
	"stackerbuild.io/stacker/pkg/types"
)

func Grab(sc types.StackerConfig, storage types.Storage, name string, source string, targetDir string,
	mode *fs.FileMode, uid, gid int,
) error {
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

	err = c.Execute(fmt.Sprintf("/static-stacker internal-go cp %s /stacker/%s", source, path.Base(source)), nil)
	if err != nil {
		return err
	}

	if mode != nil {
		err = c.Execute(fmt.Sprintf("/static-stacker internal-go chmod %s /stacker/%s", fmt.Sprintf("%o", *mode), path.Base(source)), nil)
		if err != nil {
			return err
		}
	}

	if uid > 0 {
		owns := fmt.Sprintf("%d", uid)
		if gid > 0 {
			owns += fmt.Sprintf(":%d", gid)
		}

		err = c.Execute(fmt.Sprintf("/static-stacker internal-go chown %s /stacker/%s", owns, path.Base(source)), nil)
		if err != nil {
			return err
		}
	}

	return nil
}
