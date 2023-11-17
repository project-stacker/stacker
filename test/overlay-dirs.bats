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
        url: $BUSYBOX_OCI
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
        url: $BUSYBOX_OCI
    overlay_dirs:
        - source: dir_to_overlay
    run: |
        [ -f /file ]
second:
    from:
        type: built
        tag: first
    imports: stacker://first/file
    run: |
        [ -f /stacker/imports/file ]
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
        url: $BUSYBOX_OCI
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
        url: $BUSYBOX_OCI
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
    touch dir_to_overlay1/file1.txt
    mkdir dir_to_overlay2
    touch dir_to_overlay2/file2.txt
    mkdir dir_to_overlay3
    touch dir_to_overlay3/file3.txt
    cat > stacker.yaml << EOF
first:
    from:
        type: oci
        url: $BUSYBOX_OCI
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
    echo "modifying file" > dir_to_overlay1/file1.txt
    echo "modifying file" > dir_to_overlay2/file2.txt
    echo "modifying file" > dir_to_overlay3/file3.txt
    
    NO_DEBUG=1
    stacker build
    echo $output | grep "cache miss because content of 3 overlay dirs changed:"
    echo $output | grep "Changed overlay_dir:"
    # without debug should print only 2 dirs
    result=$(echo $output | grep -o /dir_to_overlay[1-3]/file[1-3].txt | wc -l)
    [ $result -eq 2 ]

    echo $output | grep "and 1 other overlay_dirs. use --debug for complete output"
    # now with debug
    echo "modifying file again" > dir_to_overlay1/file1.txt
    echo "modifying file again" > dir_to_overlay2/file2.txt
    echo "modifying file again" > dir_to_overlay3/file3.txt

    NO_DEBUG=0
    stacker build
    echo $output | grep "cache miss because content of 3 overlay dirs changed:"
    echo $output | grep "Changed overlay_dir:"
    echo $output | grep "dir_to_overlay1"
    echo $output | grep "dir_to_overlay2"
    echo $output | grep "dir_to_overlay3"
    echo $output | grep "file1.txt"
    echo $output | grep "file2.txt"
    echo $output | grep "file3.txt"
    result=$(echo $output | grep "and 1 other overlay_dirs. use --debug for complete output" || echo "empty")
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
        url: $BUSYBOX_OCI
    overlay_dirs:
        - source: dir_to_overlay
          dest: /usr/local
    run: |
        [ -f /usr/local/file ]
        # -c == --format, but busybox does not support --format
        [ "\$(stat -c%G /usr/local/file )" == "root" ]
        [ "\$(stat -c%U /usr/local/file )" == "root" ]
EOF
    stacker build

    umoci unpack --image oci:first dest
    [ -f dest/rootfs/usr/local/file ]
    [ -f dest/rootfs/usr/local/file2 ]
}
