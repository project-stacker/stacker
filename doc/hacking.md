## Hacking stacker

The first step to trying to find a bug in stacker is to run it with --debug.
This will give you a stack trace from where (at least in stacker's code) the
error originated via [github.com/pkg/errors](https://github.com/pkg/errors).

Sometimes it is useful to write a small reproducer in `test/`, and run it with:

    make check TEST=myreproducer.bats

## Overlayfs / layer issues

Another thing `--debug` will show you is what overlay arguments it is sending
to LXC. Note that the build overlay filesystem never exists in the host mount
namespace, but is created by liblxc in the child namespace. Sometimes it can be
useful to take these overlay args and split up the lowerdirs:

    /data/ssd/builds/stacker/stacker_layers/.roots/sha256_f8e46c301da6347e78057d8fe48a6bbd8fc0cab213d47825f5c0c0646f542b6b/overlay
    /data/ssd/builds/stacker/stacker_layers/.roots/sha256_7eb8e296d351fe6d0c87fea979b305e2b1f19548d99f9aee4b8030b596f02efd/overlay
    /data/ssd/builds/stacker/stacker_layers/.roots/sha256_ca379e914166030218007477a7b9cfd0ca3dd554c58e2401c58c634fac9182f8/overlay

and look through each one (top to bottom, as the overlay stack would present)
in order to see what's going on.

## Debugging LXC

If things are really bad, you may end up wading through liblxc. With `--debug`,
stacker will also try and render any liblxc ERRORs to stdout, but sometimes it
can be useful to see a full liblxc trace log. This is available in
`$(--stacker-dir)/lxc.log` for the last run.


If you get even more in the weeds, you may need to build your own liblxc with
debug statements. Thankfully, everything is statically linked so this is fairly
easy to test locally, as long as your host liblxc can build stacker:

    make LXC_CLONE_URL=https://github.com/tych0/lxc LXC_BRANCH=my-debug-branch

Stacker links against this through a convoluted mechanism: it builds a static C
program in `/cmd/lxc-wrapper/` that takes a few relevant arguments about what
mode to drive liblxc in. Stacker uses the `go-embed` mechanism to embed the
resulting statically linked binary, and then resolves and execs it at runtime
via the code in `/embed-exec`. The reason for all this indirection vs. linking
against something directly is that the kernel does not allow multithreaded
programs to unshare user namespaces. Since the go runtime spawns many threads
for GC and various other tasks, go code cannot directly unshare a user
namespace (one wonders, then, why this was the language chosen for runc, lxd,
etc...). A previous implementation (the one in lxd) was to use some
`__attribute__((constructor))` nonsense and hope for the best, but it doesn't
work in all cases, and go-embed allows for librar-ization of stacker code if
someone else wants to use it eventually. See 8fa336834f31 ("container: move to
go-embed for re-exec of C code") for details on that approach.

## Overlay storage layout

The storage parent directory is whatever is specified to stacker via
`--roots-dir`. Each layer is extracted into a `sha256_$hash/overlay` directory,
which is then sewn together via overlayfs. At the top level, for a layer called
`foo`, there are two directories: `foo/rootfs`, and `foo/overlay`. During the
build, `foo`'s rootfs is mounted inside the container as `foo/rootfs`, with the
overlay `upperdir=foo/overlay`. This way, whatever filesystem mutations the
`foo` layer's `run:` section performs end up in `foo/overlay`.

After the `run:` section, stacker generates whatever layers the user requested
from this, creates `sha256_$hash/overlay` dirs with the contents (if two layer
types were converted, then the hash of the squashfs output will just be a
symlink to the tar layer's directory to save space), and
`foo/overlay_metadata.json` will be updated to reflect these new outputs, for
use when e.g. `foo` is a dependency of some other layer `bar`.

Note that there is currently one wart. In a stacker file like:

    foo:
        from:
            type: docker
            url: docker://ubuntu:latest
        build_only: true
        run: |
            dd if=/dev/random of=/bigfile bs=1M count=1000
    bar:
        from:
            type: bult
            tag: foo
        run: |
            rm /bigfile

The final image for `bar` will actually contain a layer with `/bigfile` in it,
because the `foo` layer's mutations are generated independently of `bar`'s.
Some clever userspace overlay collapsing could be done here to remove this
wart, though.
