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
