package stacker

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"stackerbuild.io/stacker/pkg/container"
	"stackerbuild.io/stacker/pkg/types"
)

func Grab(sc types.StackerConfig, storage types.Storage, name string, source string, targetDir string,
	idest string, mode *fs.FileMode, uid, gid int,
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

	err = SetupBuildContainerConfig(sc, storage, c, name)
	if err != nil {
		return err
	}

	bcmd := []string{insideStaticStacker, "internal-go"}

	if idest == "" || source[len(source)-1:] != "/" {
		err = c.Execute(append(bcmd, "cp", source, "/stacker/"+path.Base(source)), nil)
	} else {
		err = c.Execute(append(bcmd, "cp", source, "/stacker/"), nil)
	}
	if err != nil {
		return err
	}

	if mode != nil {
		err = c.Execute(append(bcmd, "chmod", fmt.Sprintf("%o", *mode), "/stacker/"+path.Base(source)), nil)
		if err != nil {
			return err
		}
	}

	if uid > 0 {
		owns := fmt.Sprintf("%d", uid)
		if gid > 0 {
			owns += fmt.Sprintf(":%d", gid)
		}

		err = c.Execute(append(bcmd, "chown", owns, "/stacker/"+path.Base(source)), nil)
		if err != nil {
			return err
		}
	}

	return nil
}
