package stacker

import (
	"fmt"
	"os"
	"path"
)

func Grab(sc StackerConfig, name string, source string) error {
	c, err := NewContainer(sc, WorkingContainerName)
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
	defer os.Remove(path.Join(sc.RootFSDir, WorkingContainerName, "rootfs", "stacker"))

	return c.Execute(fmt.Sprintf("cp -a %s /stacker", source), nil)
}
