load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "/stacker is ro" {
    mkdir -p .stacker/imports/test
    touch .stacker/imports/test/foo
    chmod -R 777 .stacker/imports

    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        # make sure that /stacker is readonly
        grep "/stacker" /proc/mounts | grep "[[:space:]]ro[[:space:],]"

        # make sure stacker deleted the non-import
        [ ! -f /stacker/foo ]
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}

@test "two stackers can't run at the same time" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        echo hello world
EOF
    mkdir -p roots .stacker
    touch roots/.lock .stacker/.lock
    chmod 777 -R roots .stacker

    (
        flock 9
        bad_stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
        echo "${output}" | grep "couldn't acquire lock"
    ) 9<roots/.lock

    (
        flock 9
        bad_stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
        echo "${output}" | grep "couldn't acquire lock"
    ) 9<.stacker/.lock
}
