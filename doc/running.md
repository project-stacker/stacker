## Runtime environment

Stacker execs various tools in order to accomplish its goals.

For example, in order to generate squashfs images, the `mksquashfs` binary
needs to be present in `$PATH`.

`stacker` builds things in the host's network namespace, re-exports any of
`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` and their lowercase counterparts inside
the environment, and bind mounts in the host's /etc/resolv.conf. This means
that the network experience inside the container should be identical to the
network experience that is on the host. Since stacker is only used for building
images, this is safe and most intuitive for users on corporate networks with
complicated proxy and other setups. However, it does mean that packaging that
expects to be able to modify things in `/sys` will fail, since `/sys` is bind
mounted from the host's `/sys` (sysfs cannot be mounted in a network namespace
that a user doesn't own).

When running as an unprivileged user, stacker will attempt to run things inside
a user namespace owned by the user that executed the command, and will try to
map 65k user and group ids to meet the POSIX standard. This means that
`/etc/sub{u,g}id` should be configured with enough uids to map things
correctly. This configuration can be done automatically via `stacker
unpriv-setup`. See below for discussion on unprivileged use with particular
storage backends.

### What's inside the container

Note that unlike other container tools, stacker generally assumes what's inside
the container is a "sane" rootfs, i.e. it can exec `sh` to implement the `run:`
section.

### The overlay filesystem

Stacker cannot itself be backed by an underlying overlayfs, since stacker needs
to create whiteout files, and the kernel (rightfully) forbids manual creation
of whiteout files on overlay filesystems.

Additionally, here are no additional userspace dependencies required to use the
overlayfs backend.

#### The overlay backend and the kernel

For privileged use, the overlayfs backend should work on any reasonably recent
kernel (say >= 4.4).

For unprivileged use, the overlayfs backend requires one fairly new kernel
change, a3c751a50fe6 ("vfs: allow unprivileged whiteout creation"). This is
available in all kernels >= 5.8, and may be backported to some distribution
kernels. It also requires that unprivileged users be able to mount overlay
filesystems, something which is allowed in Ubuntu kernels and will be allowed in
upstream kernels as of 459c7c565ac3 ("ovl: unprivieged mounts"), which will be
released in 5.11.

Stacker has checks to ensure that it can run with all these environment
requirements, and will fail fast if it can't do something it should be able to
do.
