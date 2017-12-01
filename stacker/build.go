package main

import (
	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
)

var buildCmd = cli.Command{
	Name:   "build",
	Usage:  "builds a new OCI image from a stacker yaml file",
	Action: doBuild,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "stacker-file, f",
			Usage: "the input stackerfile",
			Value: "stacker.yaml",
		},
		cli.BoolFlag{
			Name: "leave-unladen",
			Usage: "leave the built rootfs mount after image building",
		},
	},
}

func doBuild(ctx *cli.Context) error {
	file := ctx.String("f")
	sf, err := stacker.NewStackerfile(file)
	if err != nil {
		return err
	}

	s, err := stacker.NewStorage(config)
	if err != nil {
		return err
	}
	if !ctx.Bool("leave-unladen") {
		defer s.Detach()
	}

	order, err := sf.DependencyOrder()
	if err != nil {
		return err
	}

	defer s.Delete("working")
	for _, name := range order {
		l := sf[name]

		s.Delete(".working")
		if l.From.Type == stacker.BuiltType {
			if err := s.Restore(l.From.Tag, ".working"); err != nil {
				return err
			}
		} else {
			if err := s.Create(".working"); err != nil {
				return err
			}

			err := stacker.GetBaseLayer(config, ".working", l)
			if err != nil {
				return err
			}
		}

		// TODO: stacker's import and run flags here

		if err := s.Snapshot(".working", name); err != nil {
			return err
		}
	}

	return nil
}
