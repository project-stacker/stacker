package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anuvu/stacker"
	"github.com/apex/log"
	"github.com/urfave/cli"
)

var (
	config  stacker.StackerConfig
	version = ""
	debug   = false
)

func main() {
	app := cli.NewApp()
	app.Name = "stacker"
	app.Usage = "stacker builds OCI images"
	app.Version = version
	app.Commands = []cli.Command{
		buildCmd,
		chrootCmd,
		unladeCmd,
		cleanCmd,
		inspectCmd,
		grabCmd,
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "stacker-dir",
			Usage: "set the directory for stacker's cache",
			Value: ".stacker",
		},
		cli.StringFlag{
			Name:  "oci-dir",
			Usage: "set the directory for OCI output",
			Value: "oci",
		},
		cli.StringFlag{
			Name:  "roots-dir",
			Usage: "set the directory for the rootfs output",
			Value: "roots",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable stacker debug mode",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		var err error
		config.StackerDir, err = filepath.Abs(ctx.String("stacker-dir"))
		if err != nil {
			return err
		}

		config.OCIDir, err = filepath.Abs(ctx.String("oci-dir"))
		if err != nil {
			return err
		}
		config.RootFSDir, err = filepath.Abs(ctx.String("roots-dir"))
		if err != nil {
			return err
		}

		debug = ctx.Bool("debug")
		log.SetLevel(log.DebugLevel)

		return nil
	}

	log.SetLevel(log.WarnLevel)

	if err := app.Run(os.Args); err != nil {
		format := "error: %v\n"
		if debug {
			format = "error: %+v\n"
		}

		fmt.Fprintf(os.Stderr, format, err)
		os.Exit(1)
	}
}
