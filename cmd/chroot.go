package main

import (
	"os"

	"github.com/anuvu/stacker"
	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/types"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var chrootCmd = cli.Command{
	Name:    "chroot",
	Usage:   "run a command in a chroot",
	Aliases: []string{"exec"},
	Action:  doChroot,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "stacker-file, f",
			Usage: "the input stackerfile",
			Value: "stacker.yaml",
		},
		cli.StringSliceFlag{
			Name:  "substitute",
			Usage: "variable substitution in stackerfiles, FOO=bar format",
		},
	},
	ArgsUsage: `[tag] [cmd]

<tag> is the built tag in the stackerfile to chroot to, or the first tag if
none is specified.

<cmd> is the command to run, or /bin/sh if none is specified. To specify cmd,
you must specify a tag.`,
}

func doChroot(ctx *cli.Context) error {
	s, locks, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer s.Detach()
	defer locks.Unlock()

	tag := ""
	if len(ctx.Args()) > 0 {
		tag = ctx.Args()[0]
	}

	cmd := "/bin/sh"

	if len(ctx.Args()) > 1 {
		cmd = ctx.Args()[1]
	}

	file := ctx.String("f")
	_, err = os.Stat(file)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't access %s", file)
		}

		log.Infof("couldn't find stacker file, chrooting to %s as best effort", tag)
		c, err := stacker.NewContainer(config, s, tag)
		if err != nil {
			return err
		}
		defer c.Close()
		return c.Execute(cmd, os.Stdin)
	}
	sf, err := types.NewStackerfile(file, ctx.StringSlice("substitute"))
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
	c, err := stacker.NewContainer(config, s, name)
	if err != nil {
		return err
	}
	defer c.Close()
	err = c.SetupLayerConfig(layer, name)
	if err != nil {
		return err
	}
	return c.Execute(cmd, os.Stdin)
}
