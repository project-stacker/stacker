package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/anuvu/stacker"
	"github.com/apex/log"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	config  stacker.StackerConfig
	version = ""
	debug   = false
)

func main() {
	user, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "couldn't get current user: %s", err)
		os.Exit(1)
	}

	app := cli.NewApp()
	app.Name = "stacker"
	app.Usage = "stacker builds OCI images"
	app.Version = version
	app.Commands = []cli.Command{
		buildCmd,
		recursiveBuildCmd,
		publishCmd,
		chrootCmd,
		unladeCmd,
		cleanCmd,
		inspectCmd,
		grabCmd,
		umociCmd,
		squashfsCmd,
		unprivSetupCmd,
		gcCmd,
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = path.Join(user.HomeDir, ".config", app.Name)
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
		cli.StringFlag{
			Name:  "config",
			Usage: "stacker config file with defaults",
			Value: path.Join(configDir, "conf.yaml"),
		},
	}

	app.Before = func(ctx *cli.Context) error {
		var err error

		content, err := ioutil.ReadFile(ctx.String("config"))
		if err == nil {
			err = yaml.Unmarshal(content, &config)
			if err != nil {
				return err
			}
		}

		if config.StackerDir == "" || ctx.IsSet("stacker-dir") {
			config.StackerDir = ctx.String("stacker-dir")
		}
		if config.OCIDir == "" || ctx.IsSet("oci-dir") {
			config.OCIDir = ctx.String("oci-dir")
		}
		if config.RootFSDir == "" || ctx.IsSet("roots-dir") {
			config.RootFSDir = ctx.String("roots-dir")
		}

		config.StackerDir, err = filepath.Abs(config.StackerDir)
		if err != nil {
			return err
		}

		config.OCIDir, err = filepath.Abs(config.OCIDir)
		if err != nil {
			return err
		}
		config.RootFSDir, err = filepath.Abs(config.RootFSDir)
		if err != nil {
			return err
		}

		debug = ctx.Bool("debug")
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
