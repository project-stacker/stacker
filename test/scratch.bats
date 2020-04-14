load helpers

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
