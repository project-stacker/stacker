### Runtime dependencies

Stacker has a runtime dependency as well, namely, the btrfs tools. These can be
installed on ubuntu with:

    apt install btrfs-progs

### Kernel Version

To use unprivileged stacker, you will need a kernel with user namespaces
enabled (>= 3.10). However, many features related to user namespaces have
landed since then, so it is best to use the most up to date kernel. For example
user namespaced file capabilities were introduced in kernel commit 8db6c34f1db,
which landed in 4.14-rc1. Stock rhel/centos images use file capabilities to
avoid making executables like ping setuid, and so unprivileged stacker will
need a >= 4.14 kernel to work with these images. Fortunately, the Ubuntu
kernels have these patches backported, so any ubuntu >= 16.04 will work.

### BTRFS

If you are running in a btrfs filesystem, nothing needs to be done.

If you are running in a non-btrfs filesystem, but as root, then stacker
will automatically create and mount a loopback btrfs to use.

If you are running as non-root in a non-btrfs filesystem, then you need
to prepare by running `sudo stacker unpriv-setup`. Note that you'll need to
mount this filesystem on every reboot, either by running `unpriv-setup` again,
or setting up the mount in systemd or fstab or something.
