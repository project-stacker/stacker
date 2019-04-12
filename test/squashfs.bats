load helpers

function teardown() {
    umount combined || true
    rm -rf combined || true
    umount layer0 || true
    umount layer1 || true
    rm -rf layer0 layer1 || true
    cleanup
}

@test "squashfs layer support" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /1
EOF

    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0
    [ -f layer0/bin/bash ]

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ -f layer1/1 ]
}

@test "squashfs file whiteouts" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        rm -rf /etc/selinux
        rm -f /usr/bin/ls
EOF

    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    # manually make an atomfs
    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1

    mkdir combined
    mount -t overlay -o "lowerdir=layer1:layer0" overlay combined

    # make sure directory and file whiteouts work
    [ ! -d combined/etc/selinux ]
    [ ! -f combined/usr/bin/ls ]
}

@test "squashfs + build only layers" {
    cat > stacker.yaml <<EOF
build:
    from:
        type: docker
        url: docker://centos:latest
    build_only: true
importer:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - stacker://build/bin/ls
    run: |
        /stacker/ls
EOF
    stacker build --layer-type squashfs
}
