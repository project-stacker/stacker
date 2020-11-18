load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "generate_labels generates oci labels" {
    cat > stacker.yaml <<EOF
label:
    from:
        type: oci
        url: $CENTOS_OCI
    generate_labels: |
        echo -n "rocks" > /oci-labels/meshuggah
EOF

    stacker build
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat "oci/blobs/sha256/$config" | jq -r .config.Labels.meshuggah)" = "rocks" ]
}
