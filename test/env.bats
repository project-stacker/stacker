load helpers

function teardown() {
    cleanup
}

@test "/stacker is ro" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        # make sure that /stacker is reasonly
        grep "/stacker" /proc/mounts | grep -P "\sro[\s,]"
EOF
    stacker build
}
