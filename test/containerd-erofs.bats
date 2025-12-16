load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "containerd with erofs support" {
    ${ROOT_DIR}/hack/tools/bin/containerd -c ${ROOT_DIR}/test/data/config.toml
}
