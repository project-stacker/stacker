#define _GNU_SOURCE
#include <stdio.h>
#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <signal.h>
#include <sched.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/socket.h>

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

static int map_id(char *args[])
{
	pid_t pid;
	int status;

	pid = fork();
	if (pid < 0) {
		fprintf(stderr, "fork(): %m\n");
		return -1;
	}

	if (!pid) {
		execvp(args[0], args);
		exit(1);
	}

	if (waitpid(pid, &status, 0) != pid) {
		fprintf(stderr, "huh? waited for wrong pid\n");
		return -1;
	}

	if (!WIFEXITED(status)) {
		fprintf(stderr, "child didn't exit: %x\n", status);
		return -1;
	}

	if (WEXITSTATUS(status)) {
		fprintf(stderr, "bad exit status from child: %d\n", WEXITSTATUS(status));
		return -1;
	}

	return 0;
}

static int do_usernsexec(int argc, char *argv[], int *status)
{
	pid_t pid;
	int sk_pair[2], ret, cur, group_start = -1, command_start = -1;
	char c = 'x', thepid[20];

	for (cur = 1; argv[cur] != NULL; cur++) {
		if (!strcmp(argv[cur], "g")) {
			group_start = cur;
		}

		if (!strcmp(argv[cur], "--")) {
			command_start = cur;
		}
	}

	if (command_start < 0) {
		fprintf(stderr, "no command to usernsexec found\n");
		goto out;
	}


	if (socketpair(PF_LOCAL, SOCK_SEQPACKET, 0, sk_pair) < 0) {
		fprintf(stderr, "socketpair(): %m\n");
		return -1;
	}

	pid = fork();
	if (pid < 0) {
		close(sk_pair[0]);
		close(sk_pair[1]);
		fprintf(stderr, "fork(): %m\n");
		return -1;
	}

	if (!pid) {
		close(sk_pair[0]);

		if (unshare(CLONE_NEWUSER | CLONE_NEWNS)) {
			fprintf(stderr, "unshare: %m\n");
			close(sk_pair[1]);
			exit(1);
		}

		// tell the parent we unshared
		if (write(sk_pair[1], &c, 1) != 1) {
			fprintf(stderr, "write: %m\n");
			close(sk_pair[1]);
			exit(1);
		}

		// wait for the parent to map our ids
		if (read(sk_pair[1], &c, 1) != 1) {
			fprintf(stderr, "child read(): %m\n");
			close(sk_pair[1]);
			exit(1);
		}
		close(sk_pair[1]);

		// let's party
		execvp(argv[command_start+1], argv + command_start + 1);
		fprintf(stderr, "usernsexec failed\n");
		exit(1);
	}

	close(sk_pair[1]);
	ret = -1;

	// wait for child to unshare
	if (read(sk_pair[0], &c, 1) != 1) {
		fprintf(stderr, "parent read(): %m\n");
		goto out;
	}
	sprintf(thepid, "%d", pid);

	// pretty brutal hack: we just re-use our argv array for exec-ing
	// new{u,g}idmap, placing NULLs and a few names in the right spots.
	// we assume first argument is uid maps and that at least one is always
	// present for root
	argv[0] = "newuidmap";
	argv[1] = thepid;
	argv[group_start] = NULL;
	if (map_id(argv))
		goto out;

	// if there was a group mapping, set it too
	if (group_start > 0) {
		argv[group_start-1] = "newgidmap";
		argv[group_start] = thepid;
		argv[command_start] = NULL;
		if (map_id(argv + group_start-1))
			goto out;
	}

	// tell child it's ok to exec
	if (write(sk_pair[0], &c, 1) != 1) {
		fprintf(stderr, "write(): %m\n");
		goto out;
	}

	if (waitpid(pid, status, 0) != pid) {
		fprintf(stderr, "waited for wrong pid?\n");
		goto out;
	}

	ret = 0;

out:
	close(sk_pair[0]);
	return ret;
}

int main(int argc, char *argv[])
{
	if (argc < 2) {
		fprintf(stderr, "bad number of args: %d\n", argc);
		return 1;
	}

	if (!strcmp(argv[1], "spawn")) {
		int ret, status;
		char buf[4096];
		ssize_t size;
		char *name, *lxcpath, *config_path;

		if (argc != 5) {
			fprintf(stderr, "bad number of args for spawn: %d\n", argc);
			return 1;
		}


		name = argv[2];
		lxcpath = argv[3];
		config_path = argv[4];

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
	} else if (!strcmp(argv[1], "usernsexec")) {
		int ret, status;

		ret = do_usernsexec(argc-1, argv+1, &status);
		if (ret)
			exit(EXIT_FAILURE);
		if (WIFSIGNALED(status)) {
			kill(0, WTERMSIG(status));
			exit(EXIT_FAILURE);
		}
		if (!WIFEXITED(status)) {
			fprintf(stderr, "huh? child didn't exit and wasn't killed by a signal: %x", status);
			exit(EXIT_FAILURE);
		}
		exit(WEXITSTATUS(status));
	} else {
		fprintf(stderr, "unknown subcommand %s", argv[1]);
		exit(EXIT_FAILURE);
	}
}
