package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anuvu/stacker"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var unprivSetupCmd = cli.Command{
	Name:   "unpriv-setup",
	Usage:  "do the necessary unprivileged setup for stacker build to work without root",
	Action: doUnprivSetup,
	Before: beforeUnprivSetup,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "uid",
			Usage: "the user to do setup for (defaults to $SUDO_UID from env)",
			Value: os.Getenv("SUDO_UID"),
		},
		cli.StringFlag{
			Name:  "gid",
			Usage: "the group to do setup for (defaults to $SUDO_GID from env)",
			Value: os.Getenv("SUDO_GID"),
		},
	},
}

func beforeUnprivSetup(ctx *cli.Context) error {
	if ctx.String("uid") == "" {
		return fmt.Errorf("please specify --uid or run unpriv-setup with sudo")
	}

	if ctx.String("gid") == "" {
		return fmt.Errorf("please specify --gid or run unpriv-setup with sudo")
	}

	return nil
}

func recursiveChown(dir string, uid int, gid int) error {
	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return os.Chown(p, uid, gid)
	})
}

func warnAboutNewuidmap() {
	_, err := exec.LookPath("newuidmap")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: no newuidmap binary present. LXC will not work correctly.")
	}

	_, err = exec.LookPath("newgidmap")
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: no newgidmap binary present. LXC will not work correctly.")
	}
}

func addSpecificEntries(file string, name string, currentId int) error {
	content, err := ioutil.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "couldn't read %s", file)
	}

	maxAlloc := 100 * 1000

	for _, line := range strings.Split(string(content), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if parts[0] == name {
			return nil
		}

		if len(parts) != 3 {
			return errors.Errorf("invalid %s entry: %s", file, line)
		}

		thisAlloc, err := strconv.Atoi(parts[1])
		if err != nil {
			return errors.Wrapf(err, "invalid %s entry: %s", file, line)
		}

		size, err := strconv.Atoi(parts[2])
		if err != nil {
			return errors.Wrapf(err, "invalid %s entry: %s", file, line)
		}

		if thisAlloc+size > maxAlloc {
			maxAlloc = thisAlloc + size
		}

	}

	// newuidmap (and thus lxc-usernsexec, or more generally liblxc) will
	// complain if the current uid is in the subuid allocation. So if it
	// is, let's just advance the subuid allocation another 65536 uids. we
	// don't need to check if this overlaps again, since we know that
	// maxAlloc was the highest existing allocation.
	if maxAlloc <= currentId && currentId < maxAlloc+65536 {
		maxAlloc += 65536
	}

	withNewEntry := append(content, []byte(fmt.Sprintf("%s:%d:65536\n", name, maxAlloc))...)
	err = ioutil.WriteFile(file, withNewEntry, 0644)
	return errors.Wrapf(err, "couldn't write %s", file)
}

func addEtcEntriesIfNecessary(uid int, gid int) error {
	currentUser, err := user.LookupId(fmt.Sprintf("%d", uid))
	if err != nil {
		return errors.Wrapf(err, "couldn't find user for %d", uid)
	}

	err = addSpecificEntries("/etc/subuid", currentUser.Username, uid)
	if err != nil {
		return err
	}

	err = addSpecificEntries("/etc/subgid", currentUser.Username, gid)
	if err != nil {
		return err
	}

	return nil
}

func doUnprivSetup(ctx *cli.Context) error {
	_, err := os.Stat(config.StackerDir)
	if err == nil {
		return fmt.Errorf("stacker dir %s already exists, aborting setup", config.StackerDir)
	}

	uid, err := strconv.Atoi(ctx.String("uid"))
	if err != nil {
		return errors.Wrapf(err, "couldn't convert uid %s", ctx.String("uid"))
	}

	gid, err := strconv.Atoi(ctx.String("gid"))
	if err != nil {
		return errors.Wrapf(err, "couldn't convert gid %s", ctx.String("gid"))
	}

	err = os.MkdirAll(path.Join(config.StackerDir), 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(path.Join(config.RootFSDir), 0755)
	if err != nil {
		return err
	}

	size := int64(100 * 1024 * 1024 * 1024)
	err = stacker.MakeLoopbackBtrfs(path.Join(config.StackerDir, "btrfs.loop"), size, uid, gid, config.RootFSDir)
	if err != nil {
		return err
	}

	err = recursiveChown(config.StackerDir, uid, gid)
	if err != nil {
		return err
	}

	err = recursiveChown(config.RootFSDir, uid, gid)
	if err != nil {
		return err
	}

	warnAboutNewuidmap()
	return addEtcEntriesIfNecessary(uid, gid)
}
