load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "oci input and output directories can be the same" {
    cat > stacker.yaml <<EOF
output1:
    from:
        type: oci
        url: oci:centos
output2:
    from:
        type: oci
        url: oci:centos
EOF

    image_copy oci:$CENTOS_OCI oci:oci:centos
    stacker build

    name0=$(cat oci/index.json | jq -r .manifests[0].annotations.'"org.opencontainers.image.ref.name"')
    [ "$name0" == "centos" ]
    name1=$(cat oci/index.json | jq -r .manifests[1].annotations.'"org.opencontainers.image.ref.name"')
    [ "$name1" == "output1" ]
    name2=$(cat oci/index.json | jq -r .manifests[2].annotations.'"org.opencontainers.image.ref.name"')
    [ "$name2" == "output2" ]
}

@test "oci imports" {
    cat > stacker.yaml <<EOF
centos2:
    from:
        type: oci
        url: dest:centos
EOF
    image_copy oci:$CENTOS_OCI oci:dest:centos
    stacker build
    [ "$(umoci ls --layout ./oci)" == "$(printf "centos2")" ]
}

@test "oci imports colons in version" {
    cat > stacker.yaml <<EOF
centos3:
    from:
        type: oci
        url: dest:centos:0.1.1
    run:
        touch /zomg
EOF
    image_copy oci:$CENTOS_OCI oci:dest:centos:0.1.1
    stacker build
    [ "$(umoci ls --layout ./oci)" == "$(printf "centos3")" ]
    umoci unpack --image oci:centos3 dest
    [ -f dest/rootfs/zomg ]
}
