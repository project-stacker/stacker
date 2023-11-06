package main

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker/pkg/stacker"
)

var grabCmd = cli.Command{
	Name:   "grab",
	Usage:  "grabs a file from the layer's filesystem",
	Action: doGrab,
	ArgsUsage: `<tag>:<path>

<tag> is the tag in a built stacker image to extract the file from.

<path> is the path to extract (relative to /) in the image's rootfs.`,
}

func doGrab(ctx *cli.Context) error {
	s, locks, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer locks.Unlock()

	parts := strings.SplitN(ctx.Args().First(), ":", 2)
	if len(parts) < 2 {
		return errors.Errorf("invalid grab argument: %s", ctx.Args().First())
	}

	name, cleanup, err := s.TemporaryWritableSnapshot(parts[0])
	if err != nil {
		return err
	}
	defer cleanup()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	return stacker.Grab(config, s, name, parts[1], cwd, "", nil, -1, -1)
}
