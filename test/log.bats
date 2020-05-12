load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm logfile || true
}

@test "log --debug works" {
    # debug is passed by default in the tests
    stacker build --help
    echo "$output" | grep "stacker version"
}

@test "--debug and --quiet together fail" {
    bad_stacker --quiet build --help
}

@test "--quiet works" {
    run "${ROOT_DIR}/stacker" --quiet build --help
    [ -z "$(echo "$output" | grep "stacker version")" ]
}

@test "--log-file works" {
    stacker --log-file=logfile build --help
    grep "stacker version" logfile
}

@test "--progress works" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://centos:latest
EOF

    stacker --progress build
    echo "$output" | grep "Copying blob"
}

@test "no progress when not attached to a terminal" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://centos:latest
EOF

    stacker build
    [ -z "$(echo "$output" | grep "Copying blob")" ]
}
