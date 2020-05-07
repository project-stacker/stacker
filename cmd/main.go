package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/anuvu/stacker"
	stackerlog "github.com/anuvu/stacker/log"
	"github.com/apex/log"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"
)

var (
	config  stacker.StackerConfig
	version = ""
)

func shouldShowProgress(ctx *cli.Context) bool {
	/* if the user provided explicit recommendations, follow those */
	if ctx.GlobalBool("no-progress") {
		return false
	}
	if ctx.GlobalBool("progress") {
		return true
	}

	/* otherise, show it when we're attached to a terminal */
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

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
		cleanCmd,
		inspectCmd,
		grabCmd,
		umociCmd,
		squashfsCmd,
		unprivSetupCmd,
		gcCmd,
		containerSetupCmd,
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
		cli.StringFlag{
			Name:  "config",
			Usage: "stacker config file with defaults",
			Value: path.Join(configDir, "conf.yaml"),
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable stacker debug mode",
		},
		cli.BoolFlag{
			Name:  "q, quiet",
			Usage: "silence all logs",
		},
		cli.StringFlag{
			Name:  "log-file",
			Usage: "log to a file instead of stderr",
		},
	}

	/*
	 * Here's a barrel of suck: urfave/cli v1 doesn't allow for default
	 * values of boolean flags. So what we want is to append either
	 * --progress if this is not a tty, or --no-progress if it is a tty, so
	 * that we can allow for the right disabling of the thing in the right
	 * case.
	 *
	 * We don't want to convert to v2, because among other things it
	 * restricts *even more* the order of arguments and flags.
	 *
	 * see shouldShowProgress() for how we resolve whether or not to
	 * actually show it.
	 */
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	if isTerminal {
		app.Flags = append(app.Flags, cli.BoolFlag{
			Name:  "no-progress",
			Usage: "disable progress when downloading container images or files",
		})
	} else {
		app.Flags = append(app.Flags, cli.BoolFlag{
			Name:  "progress",
			Usage: "enable progress when downloading container images or files",
		})
	}

	var logFile *os.File
	// close the log file if we happen to open it
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()
	debug := false
	app.Before = func(ctx *cli.Context) error {
		logLevel := log.InfoLevel
		if ctx.Bool("debug") {
			debug = true
			logLevel = log.DebugLevel
			if ctx.Bool("quiet") {
				return errors.Errorf("debug and quiet don't make sense together")
			}
		} else if ctx.Bool("quiet") {
			logLevel = log.FatalLevel
		}

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

		var handler log.Handler
		handler = stackerlog.NewTextHandler(os.Stderr)
		if ctx.String("log-file") != "" {
			logFile, err = os.Create(ctx.String("log-file"))
			if err != nil {
				return errors.Wrapf(err, "failed to access %v", logFile)
			}
			handler = stackerlog.NewTextHandler(logFile)
		}

		stackerlog.FilterNonStackerLogs(handler, logLevel)
		stackerlog.Debugf("stacker version %s", version)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		format := "error: %v\n"
		if debug {
			format = "error: %+v\n"
		}

		fmt.Fprintf(os.Stderr, format, err)
		os.Exit(1)
	}
}
