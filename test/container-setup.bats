load helpers

function teardown() {
    cleanup
}

@test "/stacker is ro" {
    TEST_TMPDIR=$(tmpd config-args)
    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://centos:latest
    build_env:
        FOO: bar
EOF
    stacker container-setup
    mount -o loop .stacker/btrfs.loop "$TEST_TMPDIR"
    tree "$TEST_TMPDIR"
    cat "$TEST_TMPDIR/test/lxc.conf"
    grep "FOO=bar" "$TEST_TMPDIR/test/lxc.conf"
}
