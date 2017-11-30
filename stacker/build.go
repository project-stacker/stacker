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

	err = s.Init()
	if err != nil {
		return err
	}

	order, err := sf.DependencyOrder()
	if err != nil {
		return err
	}

	for _, name := range order {
		err := stacker.GetBaseLayer(config, name, sf[name])
		if err != nil {
			return err
		}
	}

	return nil
}
