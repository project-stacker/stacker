package stacker

import (
	"fmt"
	"os"
	"path"

	"github.com/anuvu/stacker/types"
)

func Grab(sc types.StackerConfig, name string, source string) error {
	c, err := NewContainer(sc, name)
	if err != nil {
		return err
	}
	defer c.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = c.bindMount(cwd, "/stacker", "")
	if err != nil {
		return err
	}
	defer os.Remove(path.Join(sc.RootFSDir, name, "rootfs", "stacker"))

	return c.Execute(fmt.Sprintf("cp -a %s /stacker", source), nil)
}
