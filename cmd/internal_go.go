package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
	"stackerbuild.io/stacker/atomfs"
	"stackerbuild.io/stacker/lib"
	"stackerbuild.io/stacker/log"
	"stackerbuild.io/stacker/overlay"
)

var internalGoCmd = cli.Command{
	Name:   "internal-go",
	Hidden: true,
	Subcommands: []cli.Command{
		cli.Command{
			Name:   "cp",
			Action: doCP,
		},
		cli.Command{
			Name:   "chmod",
			Action: doChmod,
		},
		cli.Command{
			Name:   "chown",
			Action: doChown,
		},
		cli.Command{
			Name:   "check-aa-profile",
			Action: doCheckAAProfile,
		},
		/*
		 * these are not actually used by stacker, but are entrypoints
		 * to the code for use in the test suite.
		 */
		cli.Command{
			Name:   "testsuite-check-overlay",
			Action: doTestsuiteCheckOverlay,
		},
		cli.Command{
			Name:   "copy",
			Action: doImageCopy,
		},
		cli.Command{
			Name: "atomfs",
			Subcommands: []cli.Command{
				cli.Command{
					Name:   "mount",
					Action: doAtomfsMount,
				},
				cli.Command{
					Name:   "umount",
					Action: doAtomfsUmount,
				},
			},
		},
	},
	Before: doBeforeUmociSubcommand,
}

func doBeforeUmociSubcommand(ctx *cli.Context) error {
	log.Debugf("stacker subcommand: %v", os.Args)
	return nil
}

// doTestsuiteCheckOverlay is only called from the stacker test suite to
// determine if the kernel is new enough to run the full overlay test suite as
// the user it is run as.
//
// If it can do the overlay operations it exit(0)s. It prints overlay error
// returned if it cannot, and exit(50)s in that case. This way we can test for
// that error code in the test suite, vs. a standard exit(1) or exit(2) from
// urfave/cli when bad arguments are passed in the eventuality that we refactor
// this command.
func doTestsuiteCheckOverlay(ctx *cli.Context) error {
	err := os.MkdirAll(config.RootFSDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "couldn't make rootfs dir for testsuite check")
	}

	err = overlay.Check(config)
	if err != nil {
		log.Infof("%s", err)
		os.Exit(50)
	}

	return nil
}

func doImageCopy(ctx *cli.Context) error {
	if len(ctx.Args()) != 2 {
		return errors.Errorf("wrong number of args")
	}

	return lib.ImageCopy(lib.ImageCopyOpts{
		Src:      ctx.Args()[0],
		Dest:     ctx.Args()[1],
		Progress: os.Stdout,
	})
}

func doCP(ctx *cli.Context) error {
	if len(ctx.Args()) != 2 {
		return errors.Errorf("wrong number of args")
	}

	return lib.CopyThing(
		ctx.Args()[0],
		ctx.Args()[1],
	)
}

func doChmod(ctx *cli.Context) error {
	if len(ctx.Args()) != 2 {
		return errors.Errorf("wrong number of args")
	}

	return lib.Chmod(
		ctx.Args()[0],
		ctx.Args()[1],
	)
}

func doChown(ctx *cli.Context) error {
	if len(ctx.Args()) != 2 {
		return errors.Errorf("wrong number of args")
	}

	return lib.Chown(
		ctx.Args()[0],
		ctx.Args()[1],
	)
}

func doCheckAAProfile(ctx *cli.Context) error {
	toCheck := ctx.Args()[0]
	command := fmt.Sprintf("changeprofile %s", toCheck)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid := unix.Gettid()
	aaControlFile := fmt.Sprintf("/proc/%d/attr/current", tid)

	err := os.WriteFile(aaControlFile, []byte(command), 0000)
	if err != nil {
		if os.IsNotExist(err) {
			os.Exit(52)
		}
		return errors.WithStack(err)
	}

	content, err := os.ReadFile(aaControlFile)
	if err != nil {
		return errors.WithStack(err)
	}

	if strings.TrimSpace(string(content)) != fmt.Sprintf("%s (enforce)", toCheck) {
		return errors.Errorf("profile mismatch got %s expected %s", string(content), toCheck)
	}

	return nil
}

func doAtomfsMount(ctx *cli.Context) error {
	if len(ctx.Args()) != 2 {
		return errors.Errorf("wrong number of args for mount")
	}

	tag := ctx.Args()[0]
	mountpoint := ctx.Args()[1]

	wd, err := os.Getwd()
	if err != nil {
		return errors.WithStack(err)
	}

	opts := atomfs.MountOCIOpts{
		OCIDir:                 config.OCIDir,
		MetadataPath:           path.Join(wd, "atomfs-metadata"),
		Tag:                    tag,
		Target:                 mountpoint,
		AllowMissingVerityData: true,
	}

	mol, err := atomfs.BuildMoleculeFromOCI(opts)
	if err != nil {
		return err
	}

	log.Debugf("about to mount %v", mol)

	return mol.Mount(mountpoint)
}

func doAtomfsUmount(ctx *cli.Context) error {
	if len(ctx.Args()) != 1 {
		return errors.Errorf("wrong number of args for umount")
	}

	mountpoint := ctx.Args()[0]
	return atomfs.Umount(mountpoint)
}
