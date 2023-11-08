load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "squashfs empty change no layer" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $CENTOS_OCI
child:
    from:
        type: built
        tag: parent
    run: |
        echo hello world
EOF
    stacker build --layer-type squashfs
    manifest0=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    manifest1=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    echo "$manifest0"
    echo "$manifest1"

    layers0=$(cat oci/blobs/sha256/$manifest0 | jq -r '.layers | length')
    layers1=$(cat oci/blobs/sha256/$manifest1 | jq -r '.layers | length')
    echo "$layers0"
    echo "$layers1"

    [ "$layers0" = "$layers1" ]
}

@test "tar empty change no layer" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $CENTOS_OCI
child:
    from:
        type: built
        tag: parent
    run: |
        echo hello world
EOF
    stacker build
    manifest0=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    manifest1=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    echo "$manifest0"
    echo "$manifest1"

    layers0=$(cat oci/blobs/sha256/$manifest0 | jq -r '.layers | length')
    layers1=$(cat oci/blobs/sha256/$manifest1 | jq -r '.layers | length')
    echo "$layers0"
    echo "$layers1"

    [ "$layers0" = "$layers1" ]
}

@test "an image with empty layers" {
  umoci init --layout oci
  umoci new --image oci:emptylayer
  chmod -R a+rw oci

  cat > stacker.yaml <<EOF
test_empty_layer:
    from:
        type: oci
        url: oci:emptylayer
EOF
    stacker build
}

@test "a real-world docker image with empty/filler layer" {
    cat > stacker.yaml <<EOF
image:
    from:
        type: docker
        url: docker://ghcr.io/project-stacker/grafana-oss:10.1.2-ubuntu
EOF
    stacker build
}

