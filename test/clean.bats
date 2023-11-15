load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "clean of unpriv overlay works" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $BUSYBOX_OCI
EOF
    stacker build
    stacker clean
}
