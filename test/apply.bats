load helpers

function setup() {
    cat > stacker.yaml <<EOF
a:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /a
        echo "hello" > /foo
b:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /b
        echo "hello" > /foo
both:
    from:
        type: docker
        url: docker://centos:latest
    run: cat /foo
    apply:
        - oci:oci:a
        - oci:oci:b
EOF
}

function teardown() {
    cleanup
}

@test "apply logic" {
    stacker build
    umoci unpack --image oci:both dest
    [ "$status" -eq 0 ]

    [ -f dest/rootfs/a ]
    [ -f dest/rootfs/b ]
    [ "$(cat dest/rootfs/foo)" == "$(printf "hello\n")" ]
}

@test "apply adds layers by hash" {
    stacker build

    umoci unpack --image oci:both dest
    [ "$status" -eq 0 ]

    # Now, check to make sure that the layer structure for "both" looks like
    # (from top to bottom):
    #
    # 1. centos:latest
    # 2. a
    # 3. b
    #
    # In particular, in this case there should be no "merge" layer, since
    # nothing was merged.
    manifest_a=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    manifest_b=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    manifest_both=$(cat oci/index.json | jq -r .manifests[2].digest | cut -f2 -d:)

    centos_latest=$(cat oci/blobs/sha256/$manifest_a | jq -r .layers[0].digest)
    layer_a=$(cat oci/blobs/sha256/$manifest_a | jq -r .layers[1].digest)
    layer_b=$(cat oci/blobs/sha256/$manifest_b | jq -r .layers[1].digest)

    [ "$centos_latest" = "$(cat oci/blobs/sha256/$manifest_both | jq -r .layers[0].digest)" ]
    [ "$layer_a" = "$(cat oci/blobs/sha256/$manifest_both | jq -r .layers[1].digest)" ]
    [ "$layer_b" = "$(cat oci/blobs/sha256/$manifest_both | jq -r .layers[2].digest)" ]

    [ -f dest/rootfs/a ]
    [ -f dest/rootfs/b ]
    [ "$(cat dest/rootfs/foo)" == "$(printf "hello\n")" ]
}
