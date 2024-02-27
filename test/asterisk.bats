load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "wildcards work in run section" {
    cat > stacker.yaml <<"EOF"
a:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        mkdir /mybin
        cp /bin/* /mybin
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci unpack --image oci:a dest
    [ "$status" -eq 0 ]


    for i in $(ls dest/rootfs/bin); do
        stat dest/rootfs/mybin/$i
    done
}

