package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/anuvu/stacker/log"
	"github.com/pkg/errors"
)

func warnAboutNewuidmap() {
	_, err := exec.LookPath("newuidmap")
	if err != nil {
		log.Infof("WARNING: no newuidmap binary present. LXC will not work correctly.")
	}

	_, err = exec.LookPath("newgidmap")
	if err != nil {
		log.Infof("WARNING: no newgidmap binary present. LXC will not work correctly.")
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

func UidmapSetup(uid, gid int) error {
	warnAboutNewuidmap()
	return addEtcEntriesIfNecessary(uid, gid)
}
