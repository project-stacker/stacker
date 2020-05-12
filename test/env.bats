load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "/stacker is ro" {
    mkdir -p .stacker/imports/test
    touch .stacker/imports/test/foo

    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        # make sure that /stacker is readonly
        grep "/stacker" /proc/mounts | grep -P "\sro[\s,]"

        # make sure stacker deleted the non-import
        [ ! -f /stacker/foo ]
EOF
    stacker build
}
