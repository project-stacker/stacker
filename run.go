package stacker

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
)

func Run(sc StackerConfig, name string, l *Layer) error {
	c, err := newContainer(sc, ".working")
	if err != nil {
		return err
	}

	run, err := l.getRun()
	if err != nil {
		return err
	}

	importsDir := path.Join(sc.StackerDir, "imports", name)

	script := fmt.Sprintf("#!/bin/bash\n%s", strings.Join(run, "\n"))
	if err := ioutil.WriteFile(path.Join(importsDir, ".stacker-run.sh"), []byte(script), 0755); err != nil {
		return err
	}

	err = c.bindMount(importsDir, "/stacker")
	if err != nil {
		return err
	}

	fmt.Println("running commands for", name)
	return c.execute([]string{"/stacker/.stacker-run.sh"})
}
