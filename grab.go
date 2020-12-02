package stacker

import (
	"fmt"
	"os"
	"path"

	"github.com/anuvu/stacker/types"
)

func Grab(sc types.StackerConfig, storage types.Storage, name string, source string, targetDir string) error {
	c, err := NewContainer(sc, storage, name)
	if err != nil {
		return err
	}
	defer c.Close()

	err = c.bindMount(targetDir, "/stacker", "")
	if err != nil {
		return err
	}
	defer os.Remove(path.Join(sc.RootFSDir, name, "rootfs", "stacker"))

	return c.Execute(fmt.Sprintf("cp -a %s /stacker", source), nil)
}
