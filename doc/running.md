### Runtime dependencies

Stacker has a few runtime dependencies as well, namely, `skopeo` and `umoci`.
Code for installing these on ubuntu is below:

    sudo apt-add-repository ppa:projectatomic/ppa
    sudo apt update
    sudo apt install skopeo
    go install github.com/openSUSE/umoci

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
to prepare by, with privilege, mounting a btrfs under "./roots" first.
You can see this being done in tests/main.sh:

```bash
truncate -s 100G btrfs.loop
mkfs.btrfs btrfs.loop
mkdir -p roots
sudo mount -o loop,user_subvol_rm_allowed btrfs.loop roots
sudo chown -R $(id -u):$(id -g) roots
```
