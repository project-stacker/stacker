load helpers

function teardown() {
    cleanup
    rm -rf recursive || true
}

@test "importing recursively" {
    mkdir -p recursive
    touch recursive/child
    cat > stacker.yaml <<EOF
centos:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - recursive
    run: |
        [ -d /stacker/recursive ]
        [ -f /stacker/recursive/child ]
EOF

    stacker build
}

@test "importing stacker:// recursively" {
    mkdir -p recursive
    touch recursive/child
    cat > stacker.yaml <<EOF
first:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - recursive
    run: |
        [ -d /stacker/recursive ]
        [ -f /stacker/recursive/child ]
        cp -a /stacker/recursive /recursive
second:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - stacker://first/recursive
    run: |
        [ -d /stacker/recursive ]
        [ -f /stacker/recursive/child ]
EOF

    stacker build
}
