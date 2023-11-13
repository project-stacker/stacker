load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "importing from a docker hub" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: docker
        url: oci:${BUSYBOX_OCI}
    imports:
        - https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/imports/favicon.ico /favicon.ico
layer1:
    from:
        type: built
        tag: busybox
    run:
        - rm /favicon.ico
EOF
    stacker build
    stacker grab busybox:/favicon.ico
    [ "$(sha .stacker/imports/busybox/favicon.ico)" == "$(sha favicon.ico)" ]
    umoci unpack --image oci:layer1 dest
    [ ! -f dest/rootfs/favicon.ico ]
}
