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
	"fmt"
	"os"
	"strconv"

	"github.com/anuvu/stacker"
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

	return stacker.MakeLoopbackBtrfs(file, size, uid, dest)
}
