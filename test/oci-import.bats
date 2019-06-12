load helpers

function teardown() {
    cleanup
}

@test "oci imports" {
    cat > stacker.yaml <<EOF
centos2:
    from:
        type: oci
        url: dest:centos
EOF
    skopeo --insecure-policy copy docker://centos:latest oci:dest:centos
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
    skopeo --insecure-policy copy docker://centos:latest oci:dest:centos:0.1.1
    stacker build
    [ "$(umoci ls --layout ./oci)" == "$(printf "centos3")" ]
    umoci unpack --image oci:centos3 dest
    [ -f dest/rootfs/zomg ]
}
