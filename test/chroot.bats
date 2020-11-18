load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "chroot goes to a reasonable place" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $CENTOS_OCI
    run: touch /test
EOF
    stacker build
    echo "[ -f /test ]" | stacker chroot
}
