package stacker

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func Run(sc StackerConfig, name string, command string, l *Layer, onFailure string, stdin io.Reader) error {
	c, err := newContainer(sc, ".working")
	if err != nil {
		return err
	}

	importsDir := path.Join(sc.StackerDir, "imports", name)
	if _, err := os.Stat(importsDir); err == nil {
		err = c.bindMount(importsDir, "/stacker")
		if err != nil {
			return err
		}
		defer os.Remove(path.Join(sc.RootFSDir, ".working", "rootfs", "stacker"))
	}

	err = c.bindMount("/etc/resolv.conf", "/etc/resolv.conf")
	if err != nil {
		return err
	}

	binds, err := l.ParseBinds()
	if err != nil {
		return err
	}

	for _, bind := range binds {
		parts := strings.Split(bind, "->")
		if len(parts) != 1 && len(parts) != 2 {
			return fmt.Errorf("invalid bind mount %s", bind)
		}

		source := strings.TrimSpace(parts[0])
		target := source
		if len(parts) == 2 {
			target = strings.TrimSpace(parts[1])
		}

		err = c.bindMount(source, target)
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
