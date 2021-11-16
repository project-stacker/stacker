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

int main(int argc, char *argv[])
{
	int ret, status;
	char buf[4096];
	ssize_t size;
	char *cur, *name, *lxcpath, *config_path;

	if (argc != 4) {
		fprintf(stderr, "bad number of args %d\n", argc);
		return 1;
	}

	name = argv[1];
	lxcpath = argv[2];
	config_path = argv[3];

	ret = isatty(STDIN_FILENO);
	if (ret < 0) {
		perror("isatty");
		exit(96);
	}

	// If this is non interactive, get rid of our controlling terminal,
	// since we don't want lxc's setting of ISIG to ignore user's ^Cs.
	if (!ret)
		setsid();

	status = spawn_container(name, lxcpath, config_path);

	// Try and propagate the container's exit code.
	if (WIFEXITED(status)) {
		exit(WEXITSTATUS(status));
	} else {
		kill(0, WTERMSIG(status));
		exit(EXIT_FAILURE);
	}
}
