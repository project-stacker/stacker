package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/anuvu/stacker/lib"
	"github.com/anuvu/stacker/log"
	"github.com/anuvu/stacker/overlay"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
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

	err = overlay.CanDoOverlay(config)
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

func doCheckAAProfile(ctx *cli.Context) error {
	toCheck := ctx.Args()[0]
	command := fmt.Sprintf("changeprofile %s", toCheck)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid := unix.Gettid()
	aaControlFile := fmt.Sprintf("/proc/%d/attr/current", tid)

	err := ioutil.WriteFile(aaControlFile, []byte(command), 0000)
	if err != nil {
		if os.IsNotExist(err) {
			os.Exit(52)
		}
		return errors.WithStack(err)
	}

	content, err := ioutil.ReadFile(aaControlFile)
	if err != nil {
		return errors.WithStack(err)
	}

	if strings.TrimSpace(string(content)) != fmt.Sprintf("%s (enforce)", toCheck) {
		return errors.Errorf("profile mismatch got %s expected %s", string(content), toCheck)
	}

	return nil
}
