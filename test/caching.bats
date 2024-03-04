load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "from: tar layer rebuilds on change" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: tar
        url: tar.tar
EOF
    echo -n "a" > content.txt
    tar -cf tar.tar content.txt
    stacker build
    echo -n "b" > content.txt
    rm tar.tar
    tar -cf tar.tar content.txt
    stacker build
    umoci unpack --image oci:test dest
    cat dest/rootfs/content.txt
    [ "$(cat dest/rootfs/content.txt)" == "b" ]
}

@test "from: oci layer rebuilds on change" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: oci:base
EOF
    image_copy oci:$BUSYBOX_OCI oci:oci:base
    stacker build
    image_copy oci:$UBUNTU_OCI oci:oci:base
    stacker build
    umoci unpack --image oci:test dest
    grep -q Ubuntu dest/rootfs/etc/issue
}

@test "built-type layer import caching" {
    cat > stacker.yaml <<"EOF"
build-base:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
base:
    from:
        type: built
        tag: build-base
    imports:
        - foo
    run: |
        cp /stacker/imports/foo /foo
EOF
    touch foo
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo "second time" > foo
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci unpack --image oci:base dest
    [ "$(cat dest/rootfs/foo)" == "second time" ]
}

@test "import caching" {
    cat > stacker.yaml <<"EOF"
import-cache:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - link/foo
    run: cp /stacker/imports/foo/zomg /zomg
EOF
    mkdir -p tree1/foo
    echo foo >> tree1/foo/zomg
    mkdir -p tree2/foo
    echo bar >> tree2/foo/zomg

    ln -s tree1 link
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    rm link && ln -s tree2 link
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    rm link
    umoci unpack --image oci:import-cache dest
    [ "$(sha tree2/foo/zomg)" == "$(sha dest/rootfs/zomg)" ]
}

@test "remove from a dir" {
    cat > stacker.yaml <<"EOF"
a:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - foo
    run: |
        [ -f /stacker/imports/foo/bar ]
EOF

    mkdir -p foo
    touch foo/bar
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    [ "$status" -eq 0 ]

    cat > stacker.yaml <<"EOF"
a:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - foo
    run: |
        [ ! -f /stacker/imports/foo/bar ]
EOF
    rm foo/bar
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}

@test "bind rebuilds" {
    cat > stacker.yaml <<"EOF"
bind-test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - tree1/foo/zomg
    binds:
        - ${{bind_path}} -> /root/tree2/foo
    run: |
        cp /stacker/imports/zomg /root/zomg1
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
    stacker build --substitute bind_path=${bind_path} --substitute BUSYBOX_OCI=$BUSYBOX_OCI
    out=$(stacker build --substitute bind_path=${bind_path} --substitute BUSYBOX_OCI=$BUSYBOX_OCI)
    [[ "${out}" =~ ^(.*rebuilding cached layer due to use of binds in stacker file.*)$ ]]
    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]

    # TODO: FIXME: need to change the import. If stacker re-builds exactly the
    # same layer (possible if things go fast enough that the mtimes are within
    # the same second), it will come up with the same hash and fail. For now we
    # just hack it.
    echo baz >> tree2/foo/zomg
    # The layer should be rebuilt since the there is a bind configuration in stacker.yaml
    stacker build --substitute bind_path=${bind_path} --substitute BUSYBOX_OCI=$BUSYBOX_OCI
    [[ "${output}" =~ ^(.*filesystem bind-test built successfully)$ ]]
    [[ ! "${output}" =~ ^(.*found cached layer bind-test)$ ]]
}

@test "mode change is re-imported" {
    cat > stacker.yaml <<"EOF"
mode-test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - executable
    run: cp /stacker/imports/executable /executable
EOF
    touch executable
    cat stacker.yaml
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    chmod +x executable
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    umoci unpack --image oci:mode-test dest
    [ -x dest/rootfs/executable ]
}

@test "can read previous version's cache" {
    skip "old version does not support imports (plural) directive"
    # some additional testing that the cache can be read by older versions of
    # stacker (cache_test.go has the full test for the type, this just checks
    # the mechanics of filepaths and such)
    local relurl="https://github.com/project-stacker/stacker/releases/download"
    local oldver="v1.0.0-rc5"
    local oldbin="./stacker-$oldver"
    if [ "$PRIVILEGE_LEVEL" != "priv" ]; then
        skip_if_no_unpriv_overlay
    fi

    wget -O "$oldbin" --progress=dot:mega "$relurl/$oldver/stacker"
    chmod 755 "$oldbin"

    touch foo
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - foo
    run: cp /stacker/imports/foo /foo
EOF

    run_as "$oldbin" --debug build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}

@test "different old cache version is ok" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo '{"version": 1, "cache": "lolnope"}' > .stacker/build.cache
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}
