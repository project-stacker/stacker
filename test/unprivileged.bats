load helpers

function setup() {
    stacker_setup
    stacker unpriv-setup
}

function teardown() {
    cleanup
}

@test "file with chmod 000 works" {
    [ -z "$TRAVIS" ] || skip "skipping unprivileged test in travis"
    require_storage btrfs # TODO: uncomment this when more people have >= 5.8 kernel

    cat > stacker.yaml <<EOF
parent:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /etc/000
        chmod 000 /etc/000
child:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        echo "zomg" > /etc/000
        chmod 000 /etc/000
EOF
    chown -R $SUDO_USER:$SUDO_USER .
    sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE build
    umoci unpack --image oci:parent parent
    [ -f parent/rootfs/etc/000 ]
    [ "$(stat --format="%a" parent/rootfs/etc/000)" = "0" ]

    umoci unpack --image oci:child child
    [ -f child/rootfs/etc/000 ]
    [ "$(stat --format="%a" child/rootfs/etc/000)" = "0" ]
    [ "$(cat child/rootfs/etc/000)" = "zomg" ]
}

@test "unprivileged stacker" {
    [ -z "$TRAVIS" ] || skip "skipping unprivileged test in travis"
    require_storage btrfs # TODO: uncomment this when more people have >= 5.8 kernel

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
    chown -R $SUDO_USER:$SUDO_USER .
    sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE build
    umoci unpack --image oci:layer1 dest

    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]
    [ ! -f dest/rootfs/favicon.ico ]
}

@test "unprivileged btrfs cleanup" {
    [ -z "$TRAVIS" ] || skip "skipping unprivileged test in travis"
    require_storage btrfs

    cat > stacker.yaml <<'EOF'
centos:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
EOF
    chown -R $SUDO_USER:$SUDO_USER .
    sudo -u $SUDO_USER "${ROOT_DIR}/stacker" build
    stacker clean
}

@test "unprivileged read-only imports can be re-cached" {
    [ -z "$TRAVIS" ] || skip "skipping unprivileged test in travis"
    require_storage btrfs # TODO: uncomment this when more people have >= 5.8 kernel

    mkdir -p import
    touch import/this
    chmod -w import

    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - import
EOF
    chown -R $SUDO_USER:$SUDO_USER .
    sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE build
    echo that > import/this
    sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE build
}
