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
