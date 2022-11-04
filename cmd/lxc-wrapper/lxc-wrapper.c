#define _GNU_SOURCE
#include <stdio.h>
#include <unistd.h>
#include <fcntl.h>
#include <string.h>
#include <signal.h>
#include <sched.h>
#include <errno.h>
#include <sys/mount.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/socket.h>

#include <lxc/lxccontainer.h>

#define STACK_SIZE (1024 * 1024)

struct child_args {
	char **argv;          /* Command to be executed by child, with args */
	int    sk_pair[2];    /* Socket used to synchronize parent and child */
	int    command_start;
};

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

static int childFunc (void *arg) {
	struct child_args *args = arg;
	char c = 'x';
	int ret = 0;

	close(args->sk_pair[0]);

	// The intent is that our child process is in:
	//  * user namespace for 'usernsexec' (CLONE_NEWUSER)
	//  * private (MS_PRIVATE) mount namespace always (CLONE_NEWNS)
	//  * pid namespace always (CLONE_NEWPID)
	//
	// If not in its' own user namespace, then mounts will propagate.
	// change that behavior so it is consistent for both cases.
	ret = mount("none", "/", 0, MS_PRIVATE | MS_REC, NULL);
	if (ret != 0) {
		fprintf(stderr, "entering private mount namespace failed: %m\n");
		close(args->sk_pair[1]);
		exit(1);
	}

	ret = mount("proc", "/proc", "proc", 0, NULL);
	if (ret != 0) {
		fprintf(stderr, "mounting proc failed: %m\n");
		close(args->sk_pair[1]);
		exit(1);
	}

	// tell the parent we are ready.
	if (write(args->sk_pair[1], &c, 1) != 1) {
		fprintf(stderr, "write: %m\n");
		close(args->sk_pair[1]);
		exit(1);
	}

	// wait for the parent to map our ids
	if (read(args->sk_pair[1], &c, 1) != 1) {
		fprintf(stderr, "child read(): %m\n");
		close(args->sk_pair[1]);
		exit(1);
	}

	close(args->sk_pair[1]);

	// let's party
	execvp(args->argv[args->command_start], args->argv + args->command_start);
	fprintf(stderr, "usernsexec failed\n");
	exit(1);
}


static int do_nsexec(char* mode, int argc, char *argv[], int *status)
{
	pid_t pid;
	int ret, cur, group_start = -1, command_start = -1;
	char c = 'x', thepid[20];
	static char child_stack[STACK_SIZE];
	struct child_args args;

	// userns - should a userns be used?
	int userns = 0;
	int flags = 0;
	flags |= CLONE_NEWNS;
	flags |= CLONE_NEWPID;

	if (!strcmp(mode, "usernsexec")) {
		userns = 1;
		flags |= CLONE_NEWUSER;
		// usernsexec args are:
		//   u nsUidStart hostUidStart range [...]
		//	 [g [nsGidStart hostGidStart range ]...]
		//	 -- command args
		for (cur = 1; argv[cur] != NULL; cur++) {
			if (!strcmp(argv[cur], "g")) {
				group_start = cur;
			}

			if (!strcmp(argv[cur], "--")) {
				command_start = cur;
			}
		}
	} else if (!strcmp(mode, "nsexec")) {
		for (cur = 1; argv[cur] != NULL; cur++) {
			if (!strcmp(argv[cur], "--")) {
				command_start = cur;
			}
		}
	} else {
		fprintf(stderr, "Invalid mode '%s'\n", mode);
		return -1;
	}

	if (command_start < 0) {
		fprintf(stderr, "no command to %s found ('--' is required)\n", mode);
		goto out;
	}

	if (socketpair(PF_LOCAL, SOCK_SEQPACKET, 0, args.sk_pair) < 0) {
		fprintf(stderr, "socketpair(): %m\n");
		return -1;
	}

	args.argv = &argv[optind];
	args.command_start = command_start;

	pid = clone(childFunc, child_stack + STACK_SIZE, flags | SIGCHLD, &args);
	if (pid < 0) {
		close(args.sk_pair[0]);
		close(args.sk_pair[1]);
		fprintf(stderr, "fork(): %m\n");
		return -1;
	}

	close(args.sk_pair[1]);
	ret = -1;

	// wait for child to unshare
	if (read(args.sk_pair[0], &c, 1) != 1) {
		fprintf(stderr, "parent read(): %m\n");
		goto out;
	}
	sprintf(thepid, "%d", pid);

	if (userns) {
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
		}

		if (map_id(argv + group_start-1))
			goto out;
	}

	// tell child it's ok to exec
	if (write(args.sk_pair[0], &c, 1) != 1) {
		fprintf(stderr, "write(): %m\n");
		goto out;
	}

	if (waitpid(pid, status, 0) != pid) {
		fprintf(stderr, "waited for wrong pid?\n");
		goto out;
	}

	ret = 0;

out:
	close(args.sk_pair[0]);
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
	} else if (!strcmp(argv[1], "usernsexec") || !strcmp(argv[1], "nsexec")) {
		int ret, status;

		ret = do_nsexec(argv[1], argc-1, argv+1, &status);
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
		fprintf(stderr, "unknown subcommand %s\n", argv[1]);
		exit(EXIT_FAILURE);
	}
}
