load helpers

function setup() {
    cat > stacker.yaml <<EOF
import-cache:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - link/foo
    run: cp /stacker/foo/foo /foo
EOF
    mkdir -p tree1/foo
    echo foo >> tree1/foo/foo
    mkdir -p tree2/foo
    echo bar >> tree2/foo/foo
}

function teardown() {
    cleanup
    rm -rf tree1 tree2 link >& /dev/null || true
}

@test "import caching" {
    ln -s tree1 link
    stacker build
    rm link && ln -s tree2 link
    stacker build
    rm link
    umoci unpack --image oci:import-cache dest
    [ "$(sha tree2/foo/foo)" == "$(sha dest/rootfs/foo)" ]
}
