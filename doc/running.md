## Runtime environment

Stacker execs various tools in order to accomplish its goals. Some recent (>
3.1.2) version of lxc-usernsexec is required for basic functionality.

Additionally, in order to generate squashfs images, the `mksquashfs` binary
needs to be present in `$PATH`.

stacker has two storage backends: an overlayfs based backend and an older (and
slower) btrfs backend. By default, stacker uses the btrfs backend, though,
because the overlayfs backend requires a very new kernel and at least one out
of tree feature that is unlikely to land in-tree soon. See below for
discussion.

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
the container is a "sane" rootfs, i.e. it can exec things like `bash` and `cp`
(`stacker://foo/bar.baz` imports require `cp`, for example).

### The overlay backend

The overlayfs backend is considerably faster than the btrfs version, because it
skips all the mtree metadata generation steps. It also extracts things in
parallel, so filesystems with many layers will be imported faster than in the
btrfs backend.

The overlay backend cannot be itself backed by an underlying overlayfs, since
stacker needs to create whiteout files, and the kernel (rightfully) forbids
manual creation of whiteout files on overlay filesystems.

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

### The btrfs backend

First, there is a runtime dependency as well, namely, the btrfs tools. These
can be installed on ubuntu with:

    apt install btrfs-progs

#### Kernel version

To use unprivileged stacker, you will need a kernel with user namespaces
enabled (>= 3.10). However, many features related to user namespaces have
landed since then, so it is best to use the most up to date kernel. For example
user namespaced file capabilities were introduced in kernel commit 8db6c34f1db,
which landed in 4.14-rc1. Stock rhel/centos images use file capabilities to
avoid making executables like ping setuid, and so unprivileged stacker will
need a >= 4.14 kernel to work with these images. Fortunately, the Ubuntu
kernels have these patches backported, so any ubuntu >= 16.04 will work.

#### Underlying filesystem

If you are running in a btrfs filesystem, nothing needs to be done.

If you are running in a non-btrfs filesystem, but as root, then stacker
will automatically create and mount a loopback btrfs to use.

If you are running as non-root in a non-btrfs filesystem, then you need
to prepare by running `sudo stacker unpriv-setup`. Note that you'll need to
mount this filesystem on every reboot, either by running `unpriv-setup` again,
or setting up the mount in systemd or fstab or something.

#### Importing squashfs images

In order to correctly import squashfs-based images using the btrfs backend,
[squashtool](https://github.com/anuvu/squashfs) is also required in `$PATH`. This
is required because tools like unsquashfs don't understand OCI style whiteouts,
and so will not extract them correctly. (One could fix this by implementing a
subsequent extrat pass to fix up overlay style whiteouts, but it would be
better to just use the overlay backend in this case.)
