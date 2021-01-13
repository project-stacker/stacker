## Tips and Tricks

#### Building a layer from scratch

There are a couple of cases where it may be useful to build a layer from
scratch. For example to derive a new base install of an OS or to build a
"tarball" type image which just carries data and will not actually be run by a
container runtime.

The way to accomplish this in stacker is to use a build only layer (i.e. a
layer that does not get emitted into the final OCI image, perhaps containing
assets or something that will be used by the final container).

The best way to accomplish this is as follows:

    build:
        from:
            type: docker
            url: docker://ubuntu:latest
        run: |
            touch /tmp/first
            touch /tmp/second
            tar -C /tmp -cv -f /contents.tar first second
        build_only: true
    contents:
        from:
            type: tar
            url: stacker://build/contents.tar

Or e.g. to bootstrap a base layer for CentoOS 7:

    build:
        from:
            type: docker
            url: docker://ubuntu:latest
        run: |
            yum -y --installroot=/rootfs --nogpgcheck install
            tar -C rootfs -zcf /rootfs.tar .
        build_only: true
    contents:
        from:
            type: tar
            url: stacker://build/rootfs.tar

These work by creating the base for the system in a build container with all
the utilities available needed to manipulate that base, and then asking stacker
to create a layer based on this tarball, without actually running anything
inside of the layer (which means e.g. absence of a shell or libc or whatever is
fine).
