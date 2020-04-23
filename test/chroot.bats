load helpers

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "chroot goes to a reasonable place" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: docker
        url: docker://centos:latest
    run: touch /test
EOF
    stacker build
    echo "[ -f /test ]" | stacker chroot
}
