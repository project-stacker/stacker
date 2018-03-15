package stacker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func Run(sc StackerConfig, name string, l *Layer, onFailure string) error {
	run, err := l.getRun()
	if err != nil {
		return err
	}

	if len(run) == 0 {
		return nil
	}

	c, err := newContainer(sc, ".working")
	if err != nil {
		return err
	}

	importsDir := path.Join(sc.StackerDir, "imports", name)

	script := fmt.Sprintf("#!/bin/bash -xe\n%s", strings.Join(run, "\n"))
	if err := ioutil.WriteFile(path.Join(importsDir, ".stacker-run.sh"), []byte(script), 0755); err != nil {
		return err
	}

	err = c.bindMount(importsDir, "/stacker")
	if err != nil {
		return err
	}
	defer os.Remove(path.Join(sc.RootFSDir, ".working", "rootfs", "stacker"))

	err = c.bindMount("/etc/resolv.conf", "/etc/resolv.conf")
	if err != nil {
		return err
	}

	fmt.Println("running commands for", name)

	// These should all be non-interactive; let's ensure that.
	err = c.execute("/stacker/.stacker-run.sh", nil)
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
