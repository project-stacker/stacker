load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "cp is not required for stacker:// imports" {
    cat > stacker.yaml <<EOF
build:
    from:
        type: docker
        url: oci:${UBUNTU_OCI}
    run: |
        touch /tmp/first
        touch /tmp/second
        tar -C /tmp -cv -f /contents.tar first second
    build_only: true
contents:
    from:
        type: tar
        url: stacker://build/contents.tar
contents2:
    from:
        type: built
        tag: build
    import:
        - stacker://contents/first
        - stacker://contents/second
    run: |
        [ -f /stacker/imports/first ]
        [ -f /stacker/imports/second ]
EOF
    stacker build
}
