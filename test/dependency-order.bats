
load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "base layer missing fails and prints" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: built
        tag: notatag
EOF
    bad_stacker build
    [[ "${output}" =~ ^(.*)$ ]]
    echo "${output}" | grep "couldn't find dependencies for test: base layer notatag"
}

@test "imports missing fails and prints" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        tag: $CENTOS_OCI
    import:
        - stacker://foo/bar
        - stacker://baz/foo
EOF
    bad_stacker build
    echo "${output}" | grep "couldn't find dependencies for test: stacker://foo/bar, stacker://baz/foo"
}
