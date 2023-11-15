load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "bad stacker:// import" {
    cat > stacker.yaml <<EOF
bad:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - stacker://idontexist/file
EOF
    bad_stacker build
}

@test "invalid yaml entry" {
    cat > stacker.yaml <<EOF
foo:
    notanentry:
        foo: bar
EOF
    bad_stacker build
}

@test "missing tag for base layer of type built" {
    cat > stacker1.yaml <<EOF
layer1:
    from:
        type: oci
        url: $BUSYBOX_OCI
EOF
    cat > stacker2.yaml <<EOF
config:
    prerequisites:
        - stacker1.yaml
layer2:
    from:
        type: built
        url: $BUSYBOX_OCI
EOF
    bad_stacker build -f stacker2.yaml
}
