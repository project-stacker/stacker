load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "overlay_dirs works" {
    mkdir dir_to_overlay
    touch dir_to_overlay/file1
    touch dir_to_overlay/file2
    touch dir_to_overlay/executable
    chmod +x dir_to_overlay/executable
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    overlay_dirs:
        - source: dir_to_overlay
    run: |
        [ -f /file1 ]
        [ -f /file2 ]
        [ -x /executable ]
EOF
    stacker build

    umoci unpack --image oci:first dest
    [ -f dest/rootfs/file1 ]
    [ -f dest/rootfs/file2 ]
    [ -x dest/rootfs/executable ]
}

@test "import from overlay_dir works" {
    mkdir dir_to_overlay
    touch dir_to_overlay/file
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    overlay_dirs:
        - source: dir_to_overlay
    run: |
        [ -f /file ]
second:
    from:
        type: built
        tag: first
    import: stacker://first/file
    run: |
        [ -f /stacker/file ]
EOF
    stacker build

    umoci unpack --image oci:first dest
    [ -f dest/rootfs/file ]
}

@test "overlay_dirs dest works" {
    mkdir dir_to_overlay
    touch dir_to_overlay/file
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    overlay_dirs:
        - source: dir_to_overlay
          dest: /usr/local
    run: |
        [ -f /usr/local/file ]
EOF
    stacker build

    umoci unpack --image oci:first dest
    [ -f dest/rootfs/usr/local/file ]
}

@test "overlay_dirs cache works" {
    mkdir dir_to_overlay
    touch dir_to_overlay/file
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    overlay_dirs:
        - source: dir_to_overlay
          dest: /usr/local
    run: |
        [ -f /usr/local/file ]
EOF
    stacker build
    stacker build
    echo $output | grep "found cached layer first"
    echo "modifying file" > dir_to_overlay/file
    stacker build
    echo $output | grep "cache miss because overlay_dir content changed"
    rm -rf roots
    stacker build
    echo $output | grep "cache miss because overlay_dir was missing"
}

@test "overlay_dirs don't preserve ownership" {
    mkdir dir_to_overlay
    touch dir_to_overlay/file
    touch dir_to_overlay/file2
    chown 1234:1234 dir_to_overlay/file
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    overlay_dirs:
        - source: dir_to_overlay
          dest: /usr/local
    run: |
        [ -f /usr/local/file ]
        [ "\$(stat --format=%G /usr/local/file )" == "root" ]
        [ "\$(stat --format=%U /usr/local/file )" == "root" ]
EOF
    stacker build

    umoci unpack --image oci:first dest
    [ -f dest/rootfs/usr/local/file ]
    [ -f dest/rootfs/usr/local/file2 ]
}
