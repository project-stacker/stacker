package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker/pkg/container"
	"stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/stacker"
	"stackerbuild.io/stacker/pkg/types"
)

var chrootCmd = cli.Command{
	Name:    "chroot",
	Usage:   "run a command in a chroot",
	Aliases: []string{"exec"},
	Action:  doChroot,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "stacker-file",
			Aliases: []string{"f"},
			Usage:   "the input stackerfile",
			Value:   "stacker.yaml",
		},
		&cli.StringSliceFlag{
			Name:  "substitute",
			Usage: "variable substitution in stackerfiles, FOO=bar format",
		},
	},
	ArgsUsage: fmt.Sprintf(`[tag] [cmd]

<tag> is the built tag in the stackerfile to chroot to, or the first tag if
none is specified.

<cmd> is the command to run, or %s if none is specified. To specify cmd,
you must specify a tag.`, stacker.DefaultShell),
}

func doChroot(ctx *cli.Context) error {
	s, locks, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer locks.Unlock()

	tag := ""
	if ctx.Args().Len() > 0 {
		tag = ctx.Args().Get(0)
	}

	cmd := []string{stacker.DefaultShell}

	if ctx.Args().Len() > 1 {
		cmd[0] = ctx.Args().Get(1)
	}

	file := ctx.String("f")
	_, err = os.Stat(file)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't access %s", file)
		}

		log.Infof("couldn't find stacker file, chrooting to %s as best effort", tag)
		c, err := container.New(config, tag)
		if err != nil {
			return err
		}
		defer c.Close()
		return c.Execute(cmd, os.Stdin)
	}
	sf, err := types.NewStackerfile(file, false, ctx.StringSlice("substitute"))
	if err != nil {
		return err
	}

	if tag == "" {
		tag = sf.FileOrder[0]
	}

	layer, ok := sf.Get(tag)
	if !ok {
		return errors.Errorf("no layer %s in stackerfile", tag)
	}

	name, cleanup, err := s.TemporaryWritableSnapshot(tag)
	if err != nil {
		return err
	}
	defer cleanup()

	log.Infof("This chroot is temporary, any changes will be destroyed when it exits.")
	c, err := container.New(config, name)
	if err != nil {
		return err
	}
	defer c.Close()

	err = stacker.SetupBuildContainerConfig(config, s, c, types.InternalStackerDir, name)
	if err != nil {
		return err
	}
	err = stacker.SetupLayerConfig(config, c, layer, types.InternalStackerDir, name)
	if err != nil {
		return err
	}

	return c.Execute(cmd, os.Stdin)
}
