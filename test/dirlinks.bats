load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "copy_up on a dirlink renders a dirlink (squashfs)" {
    cat > stacker.yaml <<"EOF"
parent:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        mkdir /dir
        ln -s /dir /link
child:
    from:
        type: built
        tag: parent
    run: |
        touch /link/test
EOF
    stacker --storage-type=overlay build --layer-type=squashfs --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)
    layer2=$(cat oci/blobs/sha256/$manifest | jq -r .layers[2].digest | cut -f2 -d:)

    echo layer1 $layer1
    echo layer2 $layer2
    ls -al roots
    ls -al roots/*/overlay/
    [ -h roots/sha256_$layer1/overlay/link ]
    [ -d roots/sha256_$layer1/overlay/dir ]

    [ ! -f roots/sha256_$layer2/overlay/link ]
    [ -d roots/sha256_$layer2/overlay/dir ]
    [ -f roots/sha256_$layer2/overlay/dir/test ]
}
