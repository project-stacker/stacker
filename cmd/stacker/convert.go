package main

import (
	"log"

	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker/pkg/stacker"
)

var convertCmd = cli.Command{
	Name:   "convert",
	Usage:  "converts a Dockerfile into a stacker yaml file (experimental, best-effort)",
	Action: doConvert,
	Flags:  initConvertFlags(),
	Before: beforeConvert,
}

func initConvertFlags() []cli.Flag {
	return append(
		initCommonConvertFlags(),
		&cli.StringFlag{
			Name:    "docker-file",
			Aliases: []string{"i"},
			Usage:   "the input Dockerfile",
			Value:   "Dockerfile",
		},
		&cli.StringFlag{
			Name:    "output-file",
			Aliases: []string{"o"},
			Usage:   "the output stacker file",
			Value:   "stacker.yaml",
		},
		&cli.StringFlag{
			Name:    "substitute-file",
			Aliases: []string{"s"},
			Usage:   "the output file containing detected substitutions",
			Value:   "stacker-subs.yaml",
		},
	)
}

func initCommonConvertFlags() []cli.Flag {
	return []cli.Flag{}
}

func beforeConvert(ctx *cli.Context) error {
	// Validate build failure arguments

	return nil
}

func newConvertArgs(ctx *cli.Context) (stacker.ConvertArgs, error) {
	args := stacker.ConvertArgs{
		Config:         config,
		Progress:       shouldShowProgress(ctx),
		InputFile:      ctx.String("docker-file"),
		OutputFile:     ctx.String("output-file"),
		SubstituteFile: ctx.String("substitute-file"),
	}
	return args, nil
}

func doConvert(ctx *cli.Context) error {
	args, err := newConvertArgs(ctx)
	if err != nil {
		return err
	}

	converter := stacker.NewConverter(&args)
	if err = converter.Convert(); err != nil {
		log.Fatalf("conversion failed: %e", err)
	}

	return nil
}
