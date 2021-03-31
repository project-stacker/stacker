load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "different URLs with same base get re-imported" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $CENTOS_OCI
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
        type: oci
        url: $CENTOS_OCI
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
        type: oci
        url: $CENTOS_OCI
    import:
        - recursive
    run: |
        [ -d /stacker/recursive ]
        [ -f /stacker/recursive/child ]
        cp -a /stacker/recursive /recursive
second:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - stacker://first/recursive
    run: |
        [ -d /stacker/recursive ]
        [ -f /stacker/recursive/child ]
EOF

    stacker build
}

@test "different import types" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    touch test_file2
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: test_file
          hash: $test_file_sha
        - test_file2
        - https://bing.com/favicon.ico
    run: |
        [ -f /stacker/test_file ]
        [ -f /stacker/test_file2 ]
        cp /stacker/test_file /test_file
    build_only: true
second:
    from:
        type: built
        tag: first
    import:
        path: stacker://first/test_file
        hash: $test_file_sha
    run: |
        [ -f /stacker/test_file ]
EOF

    stacker build
}

@test "import with unmatched hash should fail" {
    touch test_file
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: test_file
          hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b856
EOF

    bad_stacker build
    echo $output | grep "is different than the actual hash"
}

@test "invalid hash should fail" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: test_file
          hash: 1234abcdef
EOF
    bad_stacker build
    echo $output | grep "is not valid"
}

@test "case insensitive hash works" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    test_file_sha_upper=${test_file_sha^^}
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: test_file
          hash: $test_file_sha
    run: |
        cp /stacker/test_file /test_file
    build_only: true
second:
    from:
        type: built
        tag: first
    import:
        path: stacker://first/test_file
        hash: $test_file_sha_upper
    run: |
        [ -f /stacker/test_file ]
EOF

    stacker build
}

@test "invalid import " {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - "zomg"
        - - "one"
          - "two"
EOF
    bad_stacker build
}

@test "copy imports in rootfs test" {
    touch test_import
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - test_import
    run: |
        [ -f /test_import ]
EOF
    stacker build
}

@test "test import dest directive" {
    touch test_import
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: test_import
          dest: /tmp
    run: |
        [ -f /tmp/test_import ]
EOF
    stacker build
}

@test "test import dest directive creates dir" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: https://bing.com/favicon.ico
          dest: /tmp/dirThatNotExists/dir2
    run: |
        [ -f /tmp/dirThatNotExists/dir2/favicon.ico ]
EOF
    stacker build
}

@test "test import dest works with stacker:// import" {
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        path: https://bing.com/favicon.ico
        dest: /tmp/favicon
    build_only: true
second:
    from:
        type: built
        tag: first
    import:
        path: stacker://first/tmp/favicon/favicon.ico
        dest: /tmp/favicon
    run: |
        [ -f /tmp/favicon/favicon.ico ]
EOF
    stacker build
}

@test "executability is preserved" {
    touch executable
    chmod +x executable
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        path: executable
    run: |
        [ -x /executable ]
EOF
    stacker build
}
