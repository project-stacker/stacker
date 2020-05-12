load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm favicon.ico >& /dev/null || true
}

@test "build only stacker" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    import: https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
    build_only: true
layer1:
    from:
        type: built
        tag: centos
    import:
        - stacker://centos/favicon.ico
    run:
        - cp /stacker/favicon.ico /favicon2.ico
EOF
    stacker build
    umoci unpack --image oci:layer1 dest
    [ "$(sha dest/rootfs/favicon.ico)" == "$(sha dest/rootfs/favicon2.ico)" ]
    [ "$(umoci ls --layout ./oci)" == "$(printf "layer1")" ]
}

@test "stacker grab" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    import: https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
    build_only: true
layer1:
    from:
        type: built
        tag: centos
    import:
        - stacker://centos/favicon.ico
    run:
        - cp /stacker/favicon.ico /favicon2.ico
EOF
    stacker build
    stacker grab centos:/favicon.ico
    [ -f favicon.ico ]
    [ "$(sha favicon.ico)" == "$(sha .stacker/imports/centos/favicon.ico)" ]
}
