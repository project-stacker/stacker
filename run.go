package stacker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func Run(sc StackerConfig, name string, l *Layer) error {
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
	return c.execute("/stacker/.stacker-run.sh")
}
