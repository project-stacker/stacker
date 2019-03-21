package main

import (
	"fmt"
	"io/ioutil"
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

	tags, err := ioutil.ReadDir(config.RootFSDir)
	if err != nil {
		return err
	}

	if len(tags) == 0 {
		return fmt.Errorf("no builds present")
	}

	tag := tags[0].Name()

	if len(ctx.Args()) > 0 {
		tag = ctx.Args()[0]
	}

	cmd := "/bin/sh"

	if len(ctx.Args()) > 1 {
		cmd = ctx.Args()[1]
	}

	// It may be useful to do `stacker chroot .working` in order to inspect
	// the filesystem that just broke. So, let's try to support this. Since
	// we can't figure out easily which filesystem .working came from, we
	// fake an empty layer.
	if tag == ".working" {
		return stacker.Run(config, tag, cmd, &stacker.Layer{}, "", os.Stdin)
	}

	file := ctx.String("f")
	sf, err := stacker.NewStackerfile(file, ctx.StringSlice("substitute"))
	if err != nil {
		fmt.Printf("couldn't find stacker file, chrooting to %s as best effort\n", tag)
		return stacker.Run(config, tag, cmd, &stacker.Layer{}, "", os.Stdin)
	}

	layer, ok := sf.Get(tag)
	if !ok {
		return fmt.Errorf("no layer %s in stackerfile", tag)
	}

	defer s.Delete(".working")
	err = s.Restore(tag, ".working")
	if err != nil {
		return err
	}

	fmt.Println("WARNING: this chroot is temporary, any changes will be destroyed when it exits.")
	return stacker.Run(config, tag, cmd, layer, "", os.Stdin)
}
