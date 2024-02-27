load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "file with chmod 000 works" {
    cat > stacker.yaml <<"EOF"
parent:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/000
        chmod 000 /etc/000
child:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        echo "zomg" > /etc/000
        chmod 000 /etc/000
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci unpack --image oci:parent parent
    [ -f parent/rootfs/etc/000 ]
    [ "$(stat --format="%a" parent/rootfs/etc/000)" = "0" ]

    umoci unpack --image oci:child child
    [ -f child/rootfs/etc/000 ]
    [ "$(stat --format="%a" child/rootfs/etc/000)" = "0" ]
    [ "$(cat child/rootfs/etc/000)" = "zomg" ]
}

@test "unprivileged stacker" {
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/imports/favicon.ico /favicon.ico
layer1:
    from:
        type: built
        tag: busybox
    run:
        - rm /favicon.ico
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci unpack --image oci:busybox busybox
    [ "$(sha .stacker/imports/busybox/favicon.ico)" == "$(sha busybox/rootfs/favicon.ico)" ]
    umoci unpack --image oci:layer1 layer1
    [ ! -f layer1/rootfs/favicon.ico ]
}

@test "unprivileged read-only imports can be re-cached" {
    require_privilege unpriv

    sudo -s -u $SUDO_USER <<"EOF"
mkdir -p import
touch import/this
chmod -w import
EOF

    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - import
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    ls -al import import/*
    echo that | sudo -u $SUDO_USER tee import/this
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}

@test "/stacker in unprivileged mode gets deleted" {
    require_privilege unpriv

    sudo -s -u $SUDO_USER <<"EOF"
touch first
touch second
EOF

    cat > stacker.yaml <<"EOF"
base:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - first
        - second
    run: |
        ls -alh /stacker/imports
        tar -C /stacker/imports -cv -f /base.tar.gz first second
next:
    from:
        type: tar
        url: stacker://base/base.tar.gz
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    umoci unpack --image oci:base base
    [ ! -d base/rootfs/stacker ]

    umoci unpack --image oci:next next
    [ -f next/rootfs/first ]
    [ -f next/rootfs/second ]
    [ ! -d next/rootfs/stacker ]
}

@test "stacker switching privilege modes fails" {
    require_privilege unpriv

    cat > stacker.yaml <<"EOF"
base:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - test
    run: cat /stacker/imports/test
EOF
    echo unpriv | sudo -s -u $SUDO_USER tee test
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo priv > test

    # always run as privileged...
    run "${ROOT_DIR}/stacker" --debug build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo $output
    [ "$status" -ne 0 ]
}

@test "underlying layer output conversion happens in a user namespace" {
    cat > stacker.yaml <<"EOF"
image:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
EOF

    stacker build --layer-type squashfs --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)

    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0
    echo "mount has uid $(stat --format "%u" layer0/bin/mount)"
    [ "$(stat --format "%u" layer0/bin/mount)" = "0" ]
}
