/*
 * This file is a little bit strange. The problem is that we want to do
 * daemonized containers with liblxc, but we can't spawn containers in threaded
 * environments (i.e. golang), with go-lxc. So instead, we embed some C into
 * our program that catches execution before golang starts. This way, we can do
 * a tiny C program to actually spawn the container.
 */
package main

// #cgo LDFLAGS: -llxc
/*
#define _GNU_SOURCE
#include <stdio.h>
#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <signal.h>

#include <lxc/lxccontainer.h>

static int spawn_container(char *name, char *lxcpath, char *config)
{
	struct lxc_container *c;

	c = lxc_container_new(name, lxcpath);
	if (!c) {
		fprintf(stderr, "failed to create container %s\n", name);
		return -1;
	}

	c->clear_config(c);
	if (!c->load_config(c, config)) {
		fprintf(stderr, "failed to load container config at %s\n", config);
		return -1;
	}

	c->daemonize = false;
	if (!c->start(c, 1, NULL)) {
		fprintf(stderr, "failed to start container %s\n", name);
		return -1;
	}

	return c->error_num;
}

// main function for the "internal" command. Right now, arguments look like:
// stacker internal <container_name> <lxcpath> <config_path>
__attribute__((constructor)) void internal(void)
{
        int ret, status;
        char buf[4096];
        ssize_t size;
        char *cur, *name, *lxcpath, *config_path;

        ret = open("/proc/self/cmdline", O_RDONLY);
        if (ret < 0) {
                perror("error: open");
                exit(96);
        }

        if ((size = read(ret, buf, sizeof(buf)-1)) < 0) {
                close(ret);
                perror("error: read");
                exit(96);
        }
        close(ret);

	// /proc/self/cmdline is null separated, but let's be real safe
	buf[size] = 0;
	cur = buf;

#define ADVANCE_ARG		\
	do {			\
		while (*cur) {	\
			cur++;	\
		}		\
		cur++;		\
	} while (0)

	// skip argv[0]
	ADVANCE_ARG;

	// is this really the internal command, if not, continue normal execution
	if (strcmp(cur, "internal"))
		return;

	ADVANCE_ARG;
	name = cur;
	ADVANCE_ARG;
	lxcpath = cur;
	ADVANCE_ARG;
	config_path = cur;

	status = spawn_container(name, lxcpath, config_path);

	// Try and propagate the container's exit code.
	printf("error_num: %x\n", status);
	if (WIFEXITED(status)) {
		exit(WEXITSTATUS(status));
	} else {
		kill(0, WTERMSIG(status));
		exit(EXIT_FAILURE);
	}
}
*/
import "C"
