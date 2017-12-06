package stacker

import (
	"fmt"
	"path"

	"github.com/anmitsu/go-shlex"
)

func Run(sc StackerConfig, name string, run []string) error {
	c, err := newContainer(sc, ".working")
	if err != nil {
		return err
	}

	err = c.bindMount(path.Join(sc.StackerDir, "imports", name), "/stacker")
	if err != nil {
		return err
	}

	for _, cmd := range run {
		fmt.Println("running", cmd)
		args, err := shlex.Split(cmd, true)
		if err != nil {
			return err
		}

		if err := c.execute(args); err != nil {
			return err
		}
	}

	return nil
}
