load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "concurrent w aliased rootdir" {
    cat > stacker.yaml <<"EOF"
robertos:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
      sleep ${{SNOOZ}} # make sure they have time to conflict
EOF

    mkdir -p 1/roots 1/stacker 2/roots 2/stacker aliased-rootsdir
    chmod -R 777 1 2

    # simulate shared CI infra where name of sandbox dir is same despite backing store being different

    mount --bind 1/roots aliased-rootsdir
    ${ROOT_DIR}/stacker --debug --roots-dir=aliased-rootsdir --stacker-dir=1/stacker --oci-dir=1/oci --log-file=1.log build --substitute BUSYBOX_OCI=${BUSYBOX_OCI} --substitute SNOOZ=15 &

    snoozpid=$!

    sleep 5

    unshare -m bash <<EOF
        mount --bind 2/roots aliased-rootsdir
        ${ROOT_DIR}/stacker --debug --roots-dir=aliased-rootsdir --stacker-dir=2/stacker --oci-dir=2/oci --log-file=2.log \
            build --substitute BUSYBOX_OCI=${BUSYBOX_OCI} --substitute SNOOZ=0
EOF

    kill -9 $snoozpid

}
