load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "dir whiteouts work" {
    cat > stacker.yaml <<EOF
base:
    from:
        type: oci
        url: $UBUNTU_OCI
    run: |
        mkdir /hello-base
mid:
    build_only: true
    from:
        type: built
        tag: base
    run: |
        mkdir /foo
        touch /foo/bar
top:
    from:
        type: built
        tag: mid
    run: |
        rm -f /foo/bar
        rmdir /foo
EOF
    stacker build

    umoci unpack --image oci:top top
    ls top/rootfs
    [ ! -f top/rootfs/foo/bar ]
    [ ! -d top/rootfs/foo ]
}
