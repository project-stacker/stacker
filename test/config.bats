load helpers

function teardown() {
    cleanup
    rm -rf *-oci *-stacker *-roots || true
}

@test "config args work" {
    TEST_TMPDIR=$(tmpd config-args)
    local tmpd="$TEST_TMPDIR"
    cat > stacker.yaml <<EOF
test:
    from:
        type: scratch
EOF

    stacker "--oci-dir=$tmpd/args-oci" "--stacker-dir=$tmpd/args-stacker" \
        "--roots-dir=$tmpd/args-roots" build --leave-unladen
    [ -d "$tmpd/args-oci" ]
    [ -d "$tmpd/args-stacker" ]
    [ -d "$tmpd/args-roots" ]
}

@test "config file works" {
    TEST_TMPDIR=$(tmpd config-file)
    local tmpd="$TEST_TMPDIR"
    cat > stacker.yaml <<EOF
test:
    from:
        type: scratch
EOF
    cat > "$tmpd/config.yaml" <<EOF
stacker_dir: $tmpd/config-stacker
oci_dir: $tmpd/config-oci
rootfs_dir: $tmpd/config-roots
EOF

    stacker "--config=$tmpd/config.yaml" build --leave-unladen
    ls
    [ -d "$tmpd/config-oci" ]
    [ -d "$tmpd/config-stacker" ]
    [ -d "$tmpd/config-roots" ]
}

@test "config file substitutions work" {
    # the stacker file provided runs a 'my-build' that creates /my-publish/output.tar
    # output.tar's content.txt file should have the rendered values
    # for STACKER_ROOTFS_DIR STACKER_OCI_DIR and STACKER_STACKER_DIR
    #
    # my-base then imports that output file using the expansion of STACKER_ROOTFS_DIR.
    # the test compares that the rootfs from my-base contains the expected content.txt.
    TEST_TMPDIR=$(tmpd config-subs)
    local tmpd="${TEST_TMPDIR}"
    local sdir="$tmpd/stacker.d" odir="$tmpd/oci.d" rdir="$tmpd/roots.d"
    local stacker_yaml="$tmpd/stacker.yaml" config_yaml="$tmpd/config.yaml"
    local expected="$tmpd/expected.txt"

    printf "%s\n%s\n%s\n" \
        "found rootfs=$rdir" "found oci=$odir" "found stacker=$sdir" > "$expected"

    cat > "$stacker_yaml" <<"EOF"
my-build:
    build_only: true
    from:
        type: docker
        url: docker://centos:latest
    run: |
        #!/bin/sh
        set -e
        outd=/my-publish
        rm -Rf "$outd"
        mkdir -p "$outd"
        cd "$outd"
        cat > content.txt <<EOF
        found rootfs=${{STACKER_ROOTFS_DIR}}
        found oci=${{STACKER_OCI_DIR}}
        found stacker=${{STACKER_STACKER_DIR}}
        EOF
        tar -cf output.tar content.txt

my-base:
    from:
        type: tar
        url: ${{STACKER_ROOTFS_DIR}}/my-build/rootfs/my-publish/output.tar
EOF

    cat > "$config_yaml" <<EOF
stacker_dir: $sdir
oci_dir: $odir
rootfs_dir: $rdir
EOF

    stacker "--config=$config_yaml" build "--stacker-file=$stacker_yaml" --leave-unladen

    cmp_files "$expected" "$rdir/my-base/rootfs/content.txt"
}
