load helpers

function teardown() {
    cleanup
    umount args-roots || true
    umount config-roots || true
    rm -rf *-oci *-stacker *-roots config.yaml || true
}

@test "config args work" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: scratch
EOF

    stacker --oci-dir args-oci --stacker-dir args-stacker --roots-dir args-roots build --leave-unladen
    [ -d args-oci ]
    [ -d args-stacker ]
    [ -d args-roots ]
}

@test "config file works" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: scratch
EOF
    cat > config.yaml <<EOF
stacker_dir: config-stacker
oci_dir: config-oci
rootfs_dir: config-roots
EOF

    stacker --config config.yaml build --leave-unladen
    ls
    [ -d config-oci ]
    [ -d config-stacker ]
    [ -d config-roots ]
}
