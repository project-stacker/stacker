load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "multiple layer type outputs work" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $BUSYBOX_OCI
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
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $BUSYBOX_OCI
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
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $BUSYBOX_OCI
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

@test "multiple output types cache correctly" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        echo meshuggah > /rocks
EOF
    stacker build --layer-type tar --layer-type squashfs
    stacker build --layer-type tar --layer-type squashfs
    echo $output | grep "found cached layer parent"
    echo $output | grep "found cached layer parent-squashfs"
}

@test "chained built type layers are OK (tar first)" {
    cat > stacker.yaml <<EOF
one:
    from:
        type: oci
        url: $BUSYBOX_OCI
two:
    build_only: true
    from:
        type: built
        tag: one
    run: |
        echo 2 > /2
three:
    build_only: true
    from:
        type: built
        tag: two
    run: |
        echo 3 > /3
        tar -cv -f /contents.tar 2 3
four:
    from:
        type: tar
        url: stacker://three/contents.tar
five:
    from:
        type: tar
        url: stacker://three/contents.tar
EOF

    stacker build --layer-type=tar --layer-type=squashfs
    umoci unpack --image oci:four four
    [ -f four/rootfs/2 ]
    [ -f four/rootfs/3 ]

    umoci unpack --image oci:five five
    [ -f five/rootfs/2 ]
    [ -f five/rootfs/3 ]

    four_manifest=$(cat oci/index.json | jq -r .manifests[3].digest | cut -f2 -d:)
    four_lastlayer=$(cat oci/blobs/sha256/$four_manifest | jq -r .layers[-1].digest | cut -f2 -d:)

    five_manifest=$(cat oci/index.json | jq -r .manifests[5].digest | cut -f2 -d:)
    five_lastlayer=$(cat oci/blobs/sha256/$five_manifest | jq -r .layers[-1].digest | cut -f2 -d:)

    mkdir lastlayer
    mount -t squashfs oci/blobs/sha256/$five_lastlayer lastlayer
    [ -f lastlayer/2 ]
    [ -f lastlayer/3 ]
}

@test "chained built type layers are OK (squashfs first)" {
    cat > stacker.yaml <<EOF
one:
    from:
        type: oci
        url: $BUSYBOX_OCI
two:
    build_only: true
    from:
        type: built
        tag: one
    run: |
        echo 2 > /2
three:
    build_only: true
    from:
        type: built
        tag: two
    run: |
        echo 3 > /3
        tar -cv -f /contents.tar 2 3
four:
    from:
        type: tar
        url: stacker://three/contents.tar
five:
    from:
        type: tar
        url: stacker://three/contents.tar
EOF

    stacker build --layer-type=squashfs --layer-type=tar
    umoci unpack --image oci:four four
    [ -f four/rootfs/2 ]
    [ -f four/rootfs/3 ]

    umoci unpack --image oci:five five
    [ -f five/rootfs/2 ]
    [ -f five/rootfs/3 ]

    four_manifest=$(cat oci/index.json | jq -r .manifests[2].digest | cut -f2 -d:)
    four_lastlayer=$(cat oci/blobs/sha256/$four_manifest | jq -r .layers[-1].digest | cut -f2 -d:)

    five_manifest=$(cat oci/index.json | jq -r .manifests[4].digest | cut -f2 -d:)
    five_lastlayer=$(cat oci/blobs/sha256/$five_manifest | jq -r .layers[-1].digest | cut -f2 -d:)

    mkdir lastlayer
    mount -t squashfs oci/blobs/sha256/$five_lastlayer lastlayer

    [ -f lastlayer/2 ]
    [ -f lastlayer/3 ]
}
