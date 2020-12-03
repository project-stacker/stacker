load helpers

function setup() {
    stacker_setup
    unpriv_setup
}

function teardown() {
    cleanup
}

@test "file with chmod 000 works" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /etc/000
        chmod 000 /etc/000
child:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        echo "zomg" > /etc/000
        chmod 000 /etc/000
EOF
    unpriv_stacker build
    umoci unpack --image oci:parent parent
    [ -f parent/rootfs/etc/000 ]
    [ "$(stat --format="%a" parent/rootfs/etc/000)" = "0" ]

    umoci unpack --image oci:child child
    [ -f child/rootfs/etc/000 ]
    [ "$(stat --format="%a" child/rootfs/etc/000)" = "0" ]
    [ "$(cat child/rootfs/etc/000)" = "zomg" ]
}

@test "unprivileged stacker" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
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
    unpriv_stacker build
    umoci unpack --image oci:layer1 dest

    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(stacker_chroot sha /favicon.ico)" ]
    [ ! -f dest/rootfs/favicon.ico ]
}

@test "unprivileged btrfs cleanup" {
    require_storage btrfs

    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
EOF
    unpriv_stacker build
    stacker clean
}

@test "unprivileged read-only imports can be re-cached" {
    sudo -s -u $SUDO_USER <<EOF
mkdir -p import
touch import/this
chmod -w import
EOF

    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - import
EOF
    unpriv_stacker build
    ls -al import import/*
    echo that | sudo -u $SUDO_USER tee import/this
    unpriv_stacker build
}

@test "/stacker in unprivileged mode gets deleted" {
    sudo -s -u $SUDO_USER <<EOF
touch first
touch second
EOF

    cat > stacker.yaml <<EOF
base:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - first
        - second
    run: |
        ls -alh /stacker
        tar -C /stacker -cv -f /base.tar.gz first second
next:
    from:
        type: tar
        url: stacker://base/base.tar.gz
EOF
    unpriv_stacker build

    umoci unpack --image oci:base base
    [ ! -d base/rootfs/stacker ]

    umoci unpack --image oci:next next
    [ -f next/rootfs/first ]
    [ -f next/rootfs/second ]
    [ ! -d next/rootfs/stacker ]
}
