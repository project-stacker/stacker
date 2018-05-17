load helpers

function setup() {
    cat > stacker.yaml <<EOF
a:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /a
        echo "hello" > /foo
b:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /b
        echo "world" > /foo
both:
    from:
        type: docker
        url: docker://centos:latest
    run: cat /foo
    apply:
        - oci:oci:a
        - oci:oci:b
EOF
}

function teardown() {
    cleanup
}

@test "apply logic" {
    stacker build
    umoci unpack --image oci:both dest
    [ -f dest/rootfs/a ]
    [ -f dest/rootfs/b ]
    [ "$(cat dest/rootfs/foo)" == "$(printf "world\nhello\n")" ]
}
