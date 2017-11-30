package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
	"github.com/anuvu/stacker"
)

var config stacker.StackerConfig

func main() {
	app := cli.NewApp()
	app.Name = "stacker"
	app.Usage = "stacker builds OCI images"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		buildCmd,
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "stacker-dir",
			Usage: "set the directory for stacker's cache",
			Value: ".stacker",
		},
		cli.StringFlag{
			Name: "oci-dir",
			Usage: "set the directory for OCI output",
			Value: "oci",
		},
		cli.StringFlag{
			Name: "roots-dir",
			Usage: "set the directory for the rootfs output",
			Value: "roots",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		config.StackerDir = ctx.String("stacker-dir")
		config.OCIDir = ctx.String("oci-dir")
		config.RootFSDir = ctx.String("roots-dir")
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
