load helpers

function teardown() {
    cleanup
    rm -rf tree1 tree2 link foo >& /dev/null || true
}

@test "import caching" {
    cat > stacker.yaml <<EOF
import-cache:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - link/foo
    run: cp /stacker/foo/zomg /zomg
EOF
    mkdir -p tree1/foo
    echo foo >> tree1/foo/zomg
    mkdir -p tree2/foo
    echo bar >> tree2/foo/zomg

    ln -s tree1 link
    stacker build
    rm link && ln -s tree2 link
    stacker build
    rm link
    umoci unpack --image oci:import-cache dest
    [ "$(sha tree2/foo/zomg)" == "$(sha dest/rootfs/zomg)" ]
}

@test "remove from a dir" {
    cat > stacker.yaml <<EOF
a:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - foo
    run: |
        [ -f /stacker/foo/bar ]
EOF

    mkdir -p foo
    touch foo/bar
    stacker build
    [ "$status" -eq 0 ]

    cat > stacker.yaml <<EOF
a:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - foo
    run: |
        [ -f /stacker/foo/baz ]
EOF
    rm foo/bar
    touch foo/baz
    stacker build
    [ "$status" -eq 0 ]
}

@test "bind rebuilds" {
    cat > stacker.yaml <<"EOF"
bind-test:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - tree1/foo/zomg
    binds:
        - ${{bind_path}} -> /root/tree2/foo
    run: |
        cp /stacker/zomg /root/zomg1
        cp /root/tree2/foo/zomg /root/zomg2
        ls /root
EOF
    # In case the image has bind mounts, it needs to be rebuilt
    # regardless of the build cache. The reason is that tracking build
    # cache for bind mounted folders is too expensive, so we don't do it

    mkdir -p tree1/foo
    echo foo >> tree1/foo/zomg
    mkdir -p tree2/foo
    echo bar >> tree2/foo/zomg

    bind_path=$(realpath tree2/foo)

    # The layer should be built
    stacker build --substitute bind_path=${bind_path}
    out=$(stacker build --substitute bind_path=${bind_path})
    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]

    # The layer should be rebuilt since the there is a bind configuration in stacker.yaml
    out=$(stacker build --substitute bind_path=${bind_path})
    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]
    [[ ! "${out}" =~ ^(.*found cached layer bind-test)$ ]]
}
