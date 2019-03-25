load helpers

function teardown() {
    cleanup
}

@test "scratch layers" {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
EOF
    stacker build
    umoci unpack --image oci:empty dest
    [ "$(ls dest/rootfs)" == "" ]
}

@test "/bin/bash present check" {
    cat > stacker.yaml <<EOF
empty:
    from:
        type: scratch
    run: /bin/true
EOF
    bad_stacker build
    echo "$output" | grep "rootfs for empty does not have a /bin/bash"
}
