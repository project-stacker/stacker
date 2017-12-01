/*
 * stackermount is intended to be a setuid helper utility to allow most of
 * stacker to be run as an unprivileged user. stackermount's main functionality
 * is to create btrfs loopback filesystem in case it doesn't exist. It is not
 * intended to be exec'd by normal users.
 */
package main

/*
// Aah, yes, our old friend attribute constructor. Since this program is
// intended to run as setuid so that we can mount -o loop (and we fork to do
// that), we have to setuid(0);. Of course, that only affects the current
// thread. We could use runtime.LockOSThread() for this, but golang has
// hepfully made syscall.Setuid() always return ENOTSUPP. We could hardcode the
// syscall number, but this seems slightly less offensive.

#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>

__attribute__((constructor)) void init(void)
{
	if (setuid(0) < 0) {
		perror("setuid root failed");
		exit(1);
	}
}

*/
import "C"

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/freddierice/go-losetup"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 5 {
		fmt.Printf("%s <imagefile> <size> <uid> <dest>\n", os.Args[0])
		return fmt.Errorf("wrong number of arguments")
	}

	file := os.Args[1]
	size, err := strconv.ParseInt(os.Args[2], 10, 64)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return err
	}
	dest := os.Args[4]

	mounted, err := isMounted(file)
	if err != nil {
		return err
	}

	/* if it's already mounted, don't do anything */
	if mounted {
		return nil
	}

	if err := setupLoopback(file, uid, size); err != nil {
		return err
	}

	/* Now we know that file is a valid btrfs "file" and that it's
	 * not mounted, so let's mount it.
	 */
	dev, err := losetup.Attach(file, 0, false)
	if err != nil {
		return fmt.Errorf("Failed to attach loop device: %v", err)
	}
	defer dev.Detach()

	err = syscall.Mount(dev.Path(), dest, "btrfs", 0, "user_subvol_rm_allowed")
	if err != nil {
		return fmt.Errorf("Failed mount fs: %v", err)
	}

	if err := os.Chown(dest, uid, uid); err != nil {
		return fmt.Errorf("couldn't chown %s: %v", dest, err)
	}

	return nil
}

func isMounted(path string) (bool, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, path) {
			return true, nil
		}
	}

	return false, nil
}

func setupLoopback(path string, uid int, size int64) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}

		return nil
	}
	defer f.Close()

	if err := f.Chown(uid, uid); err != nil {
		os.RemoveAll(f.Name())
		return err
	}

	/* TODO: make this configurable */
	if err := syscall.Ftruncate(int(f.Fd()), size); err != nil {
		os.RemoveAll(f.Name())
		return err
	}

	output, err := exec.Command("mkfs.btrfs", f.Name()).CombinedOutput()
	if err != nil {
		os.RemoveAll(f.Name())
		return fmt.Errorf("mkfs.btrfs: %s: %s", err, output)
	}

	return nil
}
