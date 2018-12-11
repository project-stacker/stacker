load helpers

function setup() {
    cat > stacker.yaml <<EOF
a:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        mkdir /mybin
        cp /bin/* /mybin
EOF
}

function teardown() {
    cleanup
}

@test "apply logic" {
    stacker build
    umoci unpack --image oci:a dest
    [ "$status" -eq 0 ]

    for i in $(ls dest/rootfs/bin); do
        stat dest/rootfs/mybin/$i
    done
}

