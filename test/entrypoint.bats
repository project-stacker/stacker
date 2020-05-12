load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "entrypoint mess" {
    cat > stacker.yaml <<EOF
base:
    from:
        type: scratch
    cmd: foo
layer1:
    from:
        type: built
        tag: base
    entrypoint: bar
layer2:
    from:
        type: built
        tag: layer1
    full_command: baz
EOF
    stacker build

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Cmd | join("")')" = "foo" ]

    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Cmd | join("")')" = "foo" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Entrypoint | join("")')" = "bar" ]

    manifest=$(cat oci/index.json | jq -r .manifests[2].digest | cut -f2 -d:)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Cmd')" = "null" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Entrypoint | join("")')" = "baz" ]
}
