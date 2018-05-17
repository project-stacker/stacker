load helpers

function setup() {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
layer1:
    from:
        type: built
        tag: centos
    run:
        - rm /favicon.ico
EOF
}

function teardown() {
    cleanup
}

@test "importing from a docker hub" {
    stacker build --leave-unladen
    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]
    umoci unpack --image oci:layer1 dest
    [ ! -f dest/rootfs/favicon.ico ]
}
