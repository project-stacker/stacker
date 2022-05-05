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

@test "build binary and import in distroless" {
    mkdir dir_to_overlay
    chmod -R 777 dir_to_overlay
    cat > stacker.yaml << EOF
build:
    from:
        type: oci
        url: $CENTOS_OCI
    binds:
        - dir_to_overlay -> /dir_to_overlay
    run: |
        touch /dir_to_overlay/binaryfile
    build_only: true
contents:
    from:
        type: docker
        url: docker://gcr.io/distroless/base
    overlay_dirs:
        - source: dir_to_overlay
          dest: /dir_to_overlay
EOF
    stacker build
    ls dir_to_overlay | grep "binaryfile"
    
    umoci unpack --image oci:contents dest
    
    [ -f dest/rootfs/dir_to_overlay/binaryfile ]
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
    mkdir dir_to_overlay1
    touch dir_to_overlay1/file
    mkdir dir_to_overlay2
    touch dir_to_overlay2/file
    mkdir dir_to_overlay3
    touch dir_to_overlay3/file
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $CENTOS_OCI
    overlay_dirs:
        - source: dir_to_overlay1
          dest: /usr/local1
        - source: dir_to_overlay2
          dest: /usr/local2
        - source: dir_to_overlay3
          dest: /usr/local3
EOF
    stacker build
    stacker build
    echo $output | grep "found cached layer first"
    echo "modifying file" > dir_to_overlay1/file
    echo "modifying file" > dir_to_overlay2/file
    echo "modifying file" > dir_to_overlay3/file
    NO_DEBUG=1
    stacker build
    echo $output | grep "cache miss because content of 3 overlay dirs changed:"
    echo $output | grep "and 1 others. use --debug for complete output"
    echo "modifying file again" > dir_to_overlay1/file
    echo "modifying file again" > dir_to_overlay2/file
    echo "modifying file again" > dir_to_overlay3/file
    NO_DEBUG=0
    stacker build
    echo $output | grep "cache miss because content of 3 overlay dirs changed:"
    result=$(echo $output | grep "and 1 others. use --debug for complete output" || echo "empty")
    echo $result | grep "empty"
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
