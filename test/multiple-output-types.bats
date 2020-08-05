load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "multiple layer type outputs work" {
    require_storage overlay

    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        echo meshuggah > /rocks
EOF
    stacker build --layer-type tar --layer-type squashfs

    # did the tar output work?
    umoci unpack --image oci:test dest
    [ "$(cat dest/rootfs/rocks)" == "meshuggah" ]

    # and the squashfs output?
    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)
    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ "$(cat layer1/rocks)" == "meshuggah" ]
}

@test "chained multiple layer type outputs work" {
    require_storage overlay

    cat > stacker.yaml <<EOF
parent:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        echo meshuggah > /rocks
child:
    from:
        type: built
        tag: parent
    run: |
        echo primus > /sucks
EOF
    stacker build --layer-type tar --layer-type squashfs

    # did the tar output work?
    umoci unpack --image oci:child dest
    [ "$(cat dest/rootfs/rocks)" == "meshuggah" ]
    [ "$(cat dest/rootfs/sucks)" == "primus" ]

    # and the squashfs output?
    manifest=$(cat oci/index.json | jq -r .manifests[3].digest | cut -f2 -d:)

    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)
    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ "$(cat layer1/rocks)" == "meshuggah" ]

    layer2=$(cat oci/blobs/sha256/$manifest | jq -r .layers[2].digest | cut -f2 -d:)
    mkdir layer2
    mount -t squashfs oci/blobs/sha256/$layer2 layer2
    [ "$(cat layer2/sucks)" == "primus" ]
}

@test "build-only multiple layer type outputs work" {
    require_storage overlay

    cat > stacker.yaml <<EOF
parent:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        echo meshuggah > /rocks
    build_only: true
child:
    from:
        type: built
        tag: parent
    run: |
        echo primus > /sucks
EOF
    stacker build --layer-type tar --layer-type squashfs

    # did the tar output work?
    umoci unpack --image oci:child dest
    [ "$(cat dest/rootfs/rocks)" == "meshuggah" ]
    [ "$(cat dest/rootfs/sucks)" == "primus" ]

    # and the squashfs output?
    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)

    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)
    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ "$(cat layer1/rocks)" == "meshuggah" ]

    layer2=$(cat oci/blobs/sha256/$manifest | jq -r .layers[2].digest | cut -f2 -d:)
    mkdir layer2
    mount -t squashfs oci/blobs/sha256/$layer2 layer2
    [ "$(cat layer2/sucks)" == "primus" ]
}
