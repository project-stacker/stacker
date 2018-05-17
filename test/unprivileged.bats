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
    mkdir -p .stacker
    truncate -s 100G .stacker/btrfs.loop
    mkfs.btrfs .stacker/btrfs.loop
    mkdir -p roots
    mount -o loop .stacker/btrfs.loop roots
    chown -R $SUDO_USER:$SUDO_USER roots
    chown -R $SUDO_USER:$SUDO_USER .stacker
}

function teardown() {
    cleanup
}

@test "unprivileged stacker" {
    sudo -u $SUDO_USER $GOPATH/bin/stacker build
    umoci unpack --image oci:layer1 dest

    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]
    [ ! -f dest/rootfs/favicon.ico ]
}
