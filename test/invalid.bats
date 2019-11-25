load helpers

function teardown() {
    cleanup
}

@test "bad stacker:// import" {
    cat > stacker.yaml <<EOF
bad:
    from:
        type: docker
        url: docker://centos:latest
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
        type: scratch
EOF
    cat > stacker2.yaml <<EOF
config:
    prerequisites:
        - stacker1.yaml
layer2:
    from:
        type: built
        url: docker://centos:latest
EOF
    bad_stacker build -f stacker2.yaml
}
