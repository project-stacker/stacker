package main

import (
	"fmt"
	"path/filepath"
	"os"
	"os/user"

	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
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

		user, err := user.Current()
		if err != nil {
			return err
		}

		if user.Uid != "0" {
			return fmt.Errorf("must be root to use stacker")
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
