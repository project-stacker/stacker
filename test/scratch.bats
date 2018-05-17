load helpers

function setup() {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
EOF
}

function teardown() {
    cleanup
}

@test "scratch layers" {
    stacker build
    umoci unpack --image oci:empty dest
    [ "$(ls dest/rootfs)" == "" ]
}
