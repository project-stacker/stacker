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
