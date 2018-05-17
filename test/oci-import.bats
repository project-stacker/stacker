load helpers

function setup() {
    cat > stacker.yaml <<EOF
centos2:
    from:
        type: oci
        url: dest:centos
EOF
}

function teardown() {
    cleanup
}

@test "oci imports" {
    skopeo --insecure-policy copy docker://centos:latest oci:dest:centos
    stacker build
    [ "$(umoci ls --layout ./oci)" == "$(printf "centos2")" ]
}
