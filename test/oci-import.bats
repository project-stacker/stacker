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
        url: oci:busybox
output2:
    from:
        type: oci
        url: oci:busybox
EOF

    image_copy oci:$BUSYBOX_OCI oci:oci:busybox
    stacker build

    name0=$(cat oci/index.json | jq -r .manifests[0].annotations.'"org.opencontainers.image.ref.name"')
    [ "$name0" == "busybox" ]
    name1=$(cat oci/index.json | jq -r .manifests[1].annotations.'"org.opencontainers.image.ref.name"')
    [ "$name1" == "output1" ]
    name2=$(cat oci/index.json | jq -r .manifests[2].annotations.'"org.opencontainers.image.ref.name"')
    [ "$name2" == "output2" ]
}

@test "oci imports" {
    cat > stacker.yaml <<EOF
busybox2:
    from:
        type: oci
        url: dest:busybox
EOF
    image_copy oci:$BUSYBOX_OCI oci:dest:busybox
    stacker build
    [ "$(umoci ls --layout ./oci)" == "$(printf "busybox2")" ]
}

@test "oci imports colons in version" {
    cat > stacker.yaml <<EOF
busybox3:
    from:
        type: oci
        url: dest:busybox:0.1.1
    run:
        touch /zomg
EOF
    image_copy oci:$BUSYBOX_OCI oci:dest:busybox:0.1.1
    stacker build
    [ "$(umoci ls --layout ./oci)" == "$(printf "busybox3")" ]
    umoci unpack --image oci:busybox3 dest
    [ -f dest/rootfs/zomg ]
}
