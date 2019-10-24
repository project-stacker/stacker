load helpers

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "different URLs with same base get re-imported" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - https://bing.com/favicon.ico
EOF
    stacker build
    # wait, people don't use bing!
    cp .stacker/imports/thing/favicon.ico bing.ico
    sed -i -e 's/bing/google/g' stacker.yaml
    stacker build
    # we should re-import google's favicon since the URL changed
    [ "$(sha bing.ico)" != "$(sha .stacker/imports/thing/favicon.ico)" ]
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
