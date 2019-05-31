package stacker

import (
	"fmt"
	"io"
	"os"
	"path"
)

func Run(sc StackerConfig, name string, command string, l *Layer, onFailure string, stdin io.Reader) error {
	c, err := newContainer(sc, WorkingContainerName)
	if err != nil {
		return err
	}
	defer c.Close()

	importsDir := path.Join(sc.StackerDir, "imports", name)
	if _, err := os.Stat(importsDir); err == nil {
		err = c.bindMount(importsDir, "/stacker", "ro")
		if err != nil {
			return err
		}
		defer os.Remove(path.Join(sc.RootFSDir, WorkingContainerName, "rootfs", "stacker"))
	}

	err = c.bindMount("/etc/resolv.conf", "/etc/resolv.conf", "")
	if err != nil {
		return err
	}

	binds, err := l.ParseBinds()
	if err != nil {
		return err
	}

	for source, target := range binds {
		err = c.bindMount(source, target, "")
		if err != nil {
			return err
		}
	}

	// These should all be non-interactive; let's ensure that.
	err = c.execute(command, stdin)
	if err != nil {
		if onFailure != "" {
			err2 := c.execute(onFailure, os.Stdin)
			if err2 != nil {
				fmt.Printf("failed executing %s: %s\n", onFailure, err2)
			}
		}
		err = fmt.Errorf("run commands failed: %s", err)
	}

	return err
}
