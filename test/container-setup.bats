load helpers

function teardown() {
    cleanup
}

@test "container-setup generates a config" {
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
    grep "FOO=bar" "$TEST_TMPDIR/test/lxc.conf"
}
