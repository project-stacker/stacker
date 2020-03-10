package main

import (
	"fmt"
	"os"

	"github.com/anuvu/stacker"
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

	tag := stacker.WorkingContainerName

	if len(ctx.Args()) > 0 {
		tag = ctx.Args()[0]
	}

	cmd := "/bin/sh"

	if len(ctx.Args()) > 1 {
		cmd = ctx.Args()[1]
	}

	// It may be useful to do `stacker chroot _working` in order to inspect
	// the filesystem that just broke. So, let's try to support this. Since
	// we can't figure out easily which filesystem _working came from, we
	// fake an empty layer.
	if tag == stacker.WorkingContainerName {
		c, err := stacker.NewContainer(config, tag)
		if err != nil {
			return err
		}
		defer c.Close()
		return c.Execute(cmd, os.Stdin)
	}

	file := ctx.String("f")
	sf, err := stacker.NewStackerfile(file, ctx.StringSlice("substitute"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "couldn't find stacker file, chrooting to %s as best effort\n", tag)
		c, err := stacker.NewContainer(config, tag)
		if err != nil {
			return err
		}
		defer c.Close()
		return c.Execute(cmd, os.Stdin)
	}

	layer, ok := sf.Get(tag)
	if !ok {
		return fmt.Errorf("no layer %s in stackerfile", tag)
	}

	defer s.Delete(stacker.WorkingContainerName)
	err = s.Restore(tag, stacker.WorkingContainerName)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "WARNING: this chroot is temporary, any changes will be destroyed when it exits.")
	c, err := stacker.NewContainer(config, stacker.WorkingContainerName)
	if err != nil {
		return err
	}
	defer c.Close()
	err = c.SetupLayerConfig(tag, layer)
	if err != nil {
		return err
	}
	return c.Execute(cmd, os.Stdin)
}
