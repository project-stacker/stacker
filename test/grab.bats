load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

# do a build and a grab in an empty directory and
# verify that no unexpected files are created.
@test "grab has no side-effects" {
    cat > stacker.yaml <<"EOF"
layer1:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - myfile.txt
    run: |
        cp /stacker/imports/myfile.txt /my-file
EOF
    startdir="$PWD"
    wkdir="$PWD/work-dir"
    bdir="$PWD/build-dir"
    grabdir="$PWD/grab-dir"
    mkdir "$wkdir" "$grabdir" "$bdir"
    give_user_ownership "$wkdir" "$grabdir" "$bdir"

    echo "hello world" > myfile.txt
    expected_sha=$(sha myfile.txt)

    cd "$bdir"
    stacker "--work-dir=$wkdir" build "--stacker-file=$startdir/stacker.yaml" --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    dir_is_empty . ||
        test_error "build dir had unexpected files: $_RET_EXTRA"

    cd "$grabdir"
    stacker "--work-dir=$wkdir" grab layer1:/my-file
    [ -f my-file ]
    found_sha=$(sha my-file)
    [ "${expected_sha}" = "${found_sha}" ]

    dir_has_only . my-file ||
        test_error "grab produced extra files." \
            "missing=${_RET_MISSING} extra=${_RET_EXTRA}"
}

