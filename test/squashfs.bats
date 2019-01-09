load helpers

function setup() {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /1
EOF
}

function teardown() {
    umount layer0 || true
    umount layer1 || true
    rm -rf layer0 layer1 || true
    cleanup
}

@test "squashfs layer support" {
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
