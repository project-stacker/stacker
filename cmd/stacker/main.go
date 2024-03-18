package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/apex/log"
	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
	"stackerbuild.io/stacker/pkg/container"
	"stackerbuild.io/stacker/pkg/lib"
	stackerlog "stackerbuild.io/stacker/pkg/log"
	"stackerbuild.io/stacker/pkg/types"
)

var (
	config types.StackerConfig
)

func shouldShowProgress(ctx *cli.Context) bool {
	/* if the user provided explicit recommendations, follow those */
	if ctx.Bool("no-progress") {
		return false
	}
	if ctx.Bool("progress") {
		return true
	}

	/* otherise, show it when we're attached to a terminal */
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func stackerResult(err error) {
	if err != nil {
		format := "error: %v\n"
		if config.Debug {
			format = "error: %+v\n"
		}

		fmt.Fprintf(os.Stderr, format, err)

		// propagate the wrapped execution's error code if we're in the
		// userns wrapper
		exitErr, ok := errors.Cause(err).(*exec.ExitError)
		if ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

func shouldSkipInternalUserns(ctx *cli.Context) bool {
	if ctx.Args().Len() < 1 {
		// no subcommand, no need for namespace
		return true
	}
	arg0 := ctx.Args().Get(0)

	if arg0 == "internal-go" && ctx.Args().Get(1) == "testsuite-check-overlay" {
		return false
	}

	if arg0 == "bom" || arg0 == "unpriv-setup" || arg0 == "internal-go" {
		return true
	}

	return false
}

func main() {
	if !hasEmbedded {
		panic("stacker was built without embedded binaries.")
	}
	sigquits := make(chan os.Signal, 1)
	go func() {
		for range sigquits {
			debug.PrintStack()
		}
	}()
	signal.Notify(sigquits, syscall.SIGQUIT)

	app := cli.NewApp()
	app.Name = "stacker"
	app.Usage = "stacker builds OCI images"
	app.Version = fmt.Sprintf("stacker %s liblxc %s", lib.StackerVersion, lib.LXCVersion)

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir := "/home/user"
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		}

		configDir = path.Join(homeDir, ".config", app.Name)
	}

	app.Commands = []*cli.Command{
		&buildCmd,
		&bomCmd,
		&recursiveBuildCmd,
		&convertCmd,
		&publishCmd,
		&chrootCmd,
		&cleanCmd,
		&inspectCmd,
		&grabCmd,
		&internalGoCmd,
		&unprivSetupCmd,
		&gcCmd,
		&checkCmd,
	}

	app.DisableSliceFlagSeparator = true

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "work-dir",
			Usage: "set the working directory for stacker's cache, OCI output and rootfs output",
		},
		&cli.StringFlag{
			Name:  "stacker-dir",
			Usage: "set the directory for stacker's cache",
			Value: ".stacker",
		},
		&cli.StringFlag{
			Name:  "oci-dir",
			Usage: "set the directory for OCI output",
			Value: "oci",
		},
		&cli.StringFlag{
			Name:  "roots-dir",
			Usage: "set the directory for the rootfs output",
			Value: "roots",
		},
		&cli.StringFlag{
			Name:  "config",
			Usage: "stacker config file with defaults",
			Value: path.Join(configDir, "conf.yaml"),
		},
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "enable stacker debug mode",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "silence all logs",
		},
		&cli.StringFlag{
			Name:  "log-file",
			Usage: "log to a file instead of stderr",
		},
		&cli.BoolFlag{
			Name:  "log-timestamp",
			Usage: "whether to log a timestamp prefix",
		},
		&cli.StringFlag{
			Name:  "storage-type",
			Usage: "storage type (must be \"overlay\", left for compatibility)",
			Value: "overlay",
		},
		&cli.BoolFlag{
			Name:   "internal-userns",
			Usage:  "used to reexec stacker in a user namespace",
			Hidden: true,
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
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	if isTerminal {
		app.Flags = append(app.Flags, &cli.BoolFlag{
			Name:  "no-progress",
			Usage: "disable progress when downloading container images or files",
		})
	} else {
		app.Flags = append(app.Flags, &cli.BoolFlag{
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
	app.Before = func(ctx *cli.Context) error {
		logLevel := log.InfoLevel
		if ctx.Bool("debug") {
			config.Debug = true
			logLevel = log.DebugLevel
			if ctx.Bool("quiet") {
				return errors.Errorf("debug and quiet don't make sense together")
			}
		} else if ctx.Bool("quiet") {
			logLevel = log.FatalLevel
		}

		var err error
		content, err := os.ReadFile(ctx.String("config"))
		if err == nil {
			err = yaml.Unmarshal(content, &config)
			if err != nil {
				return err
			}
		}

		config.EmbeddedFS = embeddedFS

		if config.WorkDir == "" || ctx.IsSet("work-dir") {
			config.WorkDir = ctx.String("work-dir")
		}
		if config.StackerDir == "" || ctx.IsSet("stacker-dir") {
			if config.WorkDir != "" && !ctx.IsSet("stacker-dir") {
				config.StackerDir = path.Join(config.WorkDir, ctx.String("stacker-dir"))
			} else {
				config.StackerDir = ctx.String("stacker-dir")
			}
		}
		if config.OCIDir == "" || ctx.IsSet("oci-dir") {
			if config.WorkDir != "" && !ctx.IsSet("oci-dir") {
				config.OCIDir = path.Join(config.WorkDir, ctx.String("oci-dir"))
			} else {
				config.OCIDir = ctx.String("oci-dir")
			}
		}
		if config.RootFSDir == "" || ctx.IsSet("roots-dir") {
			if config.WorkDir != "" && !ctx.IsSet("roots-dir") {
				config.RootFSDir = path.Join(config.WorkDir, ctx.String("roots-dir"))
			} else {
				config.RootFSDir = ctx.String("roots-dir")
			}
		}

		// Validate roots-dir name does not contain ':'
		err = validateRootsDirName(config.RootFSDir)
		if err != nil {
			return err
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

		config.StorageType = ctx.String("storage-type")

		fi, err := os.Stat(config.CacheFile())
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			stat, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				return errors.Errorf("unknown sys stat type %T", fi.Sys())
			}

			if uint64(os.Geteuid()) != uint64(stat.Uid) {
				return errors.Errorf("previous run of stacker found with uid %d", stat.Uid)
			}
		}

		var handler log.Handler
		handler = stackerlog.NewTextHandler(os.Stderr, ctx.Bool("log-timestamp"))
		if ctx.String("log-file") != "" {
			logFile, err = os.Create(ctx.String("log-file"))
			if err != nil {
				return errors.Wrapf(err, "failed to access %v", logFile)
			}
			handler = stackerlog.NewTextHandler(logFile, ctx.Bool("log-timestamp"))
		}

		stackerlog.FilterNonStackerLogs(handler, logLevel)
		stackerlog.Debugf("stacker version %s", lib.StackerVersion)

		if !ctx.Bool("internal-userns") && !shouldSkipInternalUserns(ctx) && len(os.Args) > 1 {
			binary, err := os.Readlink("/proc/self/exe")
			if err != nil {
				return err
			}

			cmd := os.Args
			cmd[0] = binary
			cmd = append(cmd[:2], cmd[1:]...)
			cmd[1] = "--internal-userns"

			stackerResult(container.MaybeRunInNamespace(config, cmd))
		}
		return nil
	}

	stackerResult(app.Run(os.Args))
}
