load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "scratch layers" {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
EOF
    stacker build
    umoci unpack --image oci:empty dest
    [ "$(ls dest/rootfs)" == "" ]
}

@test "/bin/sh present check" {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
    run: /bin/true
EOF
    bad_stacker build
    echo "$output" | grep "does not have a /bin/sh"
}

@test "/bin/sh is a symlink check" {
    mkdir bin
    ln -s /bin/notarealshell bin/sh

    tar cvf symlink-check.tar bin

    # busybox's stuff is all a symlink to /bin/busybox, which has confused our
    # /bin/sh check in the past
    cat > stacker.yaml <<EOF
symlink-check:
    from:
        type: tar
        url: ./symlink-check.tar
    run: /bin/wontwork
EOF

    # we expect this to fail during run: (the shell is broken), but we expect
    # the shell detection to pass
    bad_stacker build
    echo "$output" | grep "run commands failed"
}

@test "derived from build_only scratch" {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
    build_only: true

empty2:
    from:
        type: built
        tag: empty
EOF
    stacker build
}

@test "derived from build_only scratch with squashfs" {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
    build_only: true

empty2:
    from:
        type: built
        tag: empty
EOF
    stacker build --layer-type squashfs
}
