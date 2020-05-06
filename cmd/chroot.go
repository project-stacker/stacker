package main

import (
	"fmt"
	"os"

	"github.com/anuvu/stacker"
	"github.com/anuvu/stacker/log"
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
	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	defer s.Detach()

	tag := ""
	if len(ctx.Args()) > 0 {
		tag = ctx.Args()[0]
	}

	cmd := "/bin/sh"

	if len(ctx.Args()) > 1 {
		cmd = ctx.Args()[1]
	}

	file := ctx.String("f")
	sf, err := stacker.NewStackerfile(file, ctx.StringSlice("substitute"))
	if err != nil {
		log.Infof("couldn't find stacker file, chrooting to %s as best effort", tag)
		c, err := stacker.NewContainer(config, tag)
		if err != nil {
			return err
		}
		defer c.Close()
		return c.Execute(cmd, os.Stdin)
	}

	if tag == "" {
		tag = sf.FileOrder[0]
	}

	layer, ok := sf.Get(tag)
	if !ok {
		return fmt.Errorf("no layer %s in stackerfile", tag)
	}

	name, cleanup, err := s.TemporaryWritableSnapshot(tag)
	if err != nil {
		return err
	}
	defer cleanup()

	log.Infof("This chroot is temporary, any changes will be destroyed when it exits.")
	c, err := stacker.NewContainer(config, name)
	if err != nil {
		return err
	}
	defer c.Close()
	err = c.SetupLayerConfig(layer)
	if err != nil {
		return err
	}
	return c.Execute(cmd, os.Stdin)
}
