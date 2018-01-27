package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/anuvu/stacker"
	"github.com/urfave/cli"
)

var (
	config  stacker.StackerConfig
	version = ""
)

func main() {
	app := cli.NewApp()
	app.Name = "stacker"
	app.Usage = "stacker builds OCI images"
	app.Version = version
	app.Commands = []cli.Command{
		buildCmd,
		unladeCmd,
		cleanCmd,
		inspectCmd,
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
			Name:   "internal-in-userns",
			Usage:  "don't use this; stacker internal only!",
			Hidden: true,
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
			fmt.Println("WARNING: rootless support experimental")
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// usernsWrapper wraps the cli Action so that it is executed in a user
// namespace with the current user's delegation.
func usernsWrapper(do func(ctx *cli.Context) error) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		if os.Geteuid() != 0 {
			user, err := user.Current()
			if err != nil {
				return err
			}

			if !ctx.Bool("internal-in-userns") {
				if stacker.IdmapSet == nil {
					return fmt.Errorf("no uidmap for %s", user.Username)
				}

				args := os.Args
				args = append(args[:1], append([]string{"--internal-in-userns"}, args[1:]...)...)
				return stacker.RunInUserns(args, "stacker re-exec")
			}
		}

		err := do(ctx)

		// Now, a convenience. Stacker just did a bunch of stuff,
		// including write files to OCIDir and StackerDir, as a uid
		// that's not the uid of the person who ran stacker. That means
		// that when they try and look at the OCI image, or delete
		// .stacker or something, they'll get EACCES. Instead, let's
		// chown everything back to their uid; this way stacker can
		// still use it the next time it runs (since it will be mapped
		// to a non-root uid inside the namespace), and the host user
		// can still manipulate files as they choose.
		//
		// Note that we don't care about errors, since this is mostly
		// for convenicence.
		doPermChange := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Chown(path, stacker.HostIDInUserns, stacker.HostIDInUserns)
		}
		filepath.Walk(config.OCIDir, doPermChange)
		filepath.Walk(config.StackerDir, doPermChange)
		return err
	}
}
