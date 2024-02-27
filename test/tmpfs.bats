load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "stacker works in a tmpfs" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        echo hello world
EOF

    mkdir -p roots
    mount -t tmpfs -o size=1G tmpfs roots
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}
