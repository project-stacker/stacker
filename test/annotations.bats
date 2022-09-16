load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "annotations work" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $CENTOS_OCI
    run: ls
    annotations:
      a.b.c.key: val
EOF
    stacker build 
    [ "$status" -eq 0 ]
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    cat oci/blobs/sha256/$manifest | jq .
    key=$(cat oci/blobs/sha256/$manifest | jq -r .annotations | cut -f1 -d:)
    echo $key
    val=$(cat oci/blobs/sha256/$manifest | jq -r .annotations | cut -f2 -d:)
    echo $val
    [[ "$key" == *"a.b.c.key"* ]]
    [[ "$val" == *"val"* ]]
}
