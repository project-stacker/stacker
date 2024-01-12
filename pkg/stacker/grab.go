package stacker

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"

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

	err = SetupBuildContainerConfig(sc, storage, c, types.InternalStackerDir, name)
	if err != nil {
		return err
	}

	idestdir := filepath.Join(types.InternalStackerDir, "grab")
	err = c.BindMount(targetDir, idestdir, "")
	if err != nil {
		return err
	}

	bcmd := []string{filepath.Join(types.InternalStackerDir, types.BinStacker), "internal-go"}

	iDestName := filepath.Join(idestdir, path.Base(source))
	if idest == "" || source[len(source)-1:] != "/" {
		err = c.Execute(append(bcmd, "cp", source, iDestName), nil)
	} else {
		err = c.Execute(append(bcmd, "cp", source, idestdir+"/"), nil)
	}
	if err != nil {
		return err
	}

	if mode != nil {
		err = c.Execute(append(bcmd, "chmod", fmt.Sprintf("%o", *mode), iDestName), nil)
		if err != nil {
			return err
		}
	}

	if uid > 0 {
		owns := fmt.Sprintf("%d", uid)
		if gid > 0 {
			owns += fmt.Sprintf(":%d", gid)
		}

		err = c.Execute(append(bcmd, "chown", owns, iDestName), nil)
		if err != nil {
			return err
		}
	}

	return nil
}
