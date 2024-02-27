load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "chroot goes to a reasonable place" {
    cat > stacker.yaml <<"EOF"
thing:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: touch /test
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo "[ -f /test ]" | stacker chroot --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}
