package main

import (
	"path"
	"strings"

	"github.com/anuvu/stacker"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)


var grabCmd = cli.Command{
	Name:   "grab",
	Usage:  "grabs a file from the layer's filesystem",
	Action: doGrab,
}

func doGrab(ctx *cli.Context) error {
	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer s.Detach()

	parts := strings.SplitN(ctx.Args().First(), ":", 2)
	if len(parts) < 2 {
		return errors.Errorf("invalid grab argument: %s", ctx.Args().First())
	}

	err = s.Restore(parts[0], ".working")
	if err != nil {
		return err
	}
	defer s.Delete(".working")

	return stacker.Grab(config, parts[0], parts[1])
}
