load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "multi-arch/os support" {
    cat > stacker.yaml <<EOF
busybox:
    os: darwin
    arch: arm64
    from:
        type: oci
        url: $BUSYBOX_OCI
    imports:
        - https://www.cisco.com/favicon.ico
EOF
    stacker build

    # check OCI image generation
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$config | jq -r '.architecture')" = "arm64" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.os')" = "darwin" ]
}

@test "multi-arch/os bad config fails" {
    cat > stacker.yaml <<EOF
busybox:
    os:
    from:
        type: oci
        url: $BUSYBOX_OCI
    imports:
        - https://www.cisco.com/favicon.ico
EOF
    bad_stacker build
    [ "$status" -eq 1 ]
}
