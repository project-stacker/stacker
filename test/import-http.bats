load helpers

function setup() {
    stacker_setup
    mkdir -p reference
    mkdir -p dest
    rm -f nm_orig
    wget http://network-test.debian.org/nm -O reference/nm_orig
    mkdir img
    # Need to separate layer download from the download of the test file for imports
    # As we want to be able to have network for base image download
    # in img/stacker1.yaml but disconnect the network for test file download
    cat > img/stacker1.yaml <<EOF
centos_base:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        ls
EOF
    cat > img/stacker2.yaml <<EOF
img:
    from:
        type: oci
        url: $(pwd)/oci:centos_base
    import:
        - http://network-test.debian.org/nm
    run: |
        cp /stacker/nm /root/nm
EOF
}

function teardown() {
    cleanup
    rm -rf img || true
    rm -rf reference || true
    rm -rf dest || true
    ip netns del stacker-test || true
}

@test "importing from cache works for unreachable http urls" {
    require_privilege priv

    # Build base image
    stacker build -f img/stacker1.yaml
    umoci ls --layout oci
    # First execution creates the cache
    stacker build -f img/stacker2.yaml
    umoci ls --layout oci
    # Second execution reads from the cache, but cannot access the net
    ip netns add stacker-test
    run ip netns exec stacker-test "${ROOT_DIR}/stacker" build -f img/stacker2.yaml
    echo $output
    [ "$status" -eq 0 ]
    umoci ls --layout oci
    umoci unpack --image oci:img dest/img
    [ "$(sha reference/nm_orig)" == "$(sha .stacker/imports/img/nm)" ]
    [ "$(sha reference/nm_orig)" == "$(sha dest/img/rootfs/root/nm)" ]
}

@test "importing cached file from http url with matching length" {
    # Build base image
    stacker build -f img/stacker1.yaml
    umoci ls --layout oci
    # First execution creates the cache
    stacker build -f img/stacker2.yaml
    umoci ls --layout oci
    # Second execution reads from the cache
    stacker build -f img/stacker2.yaml
    umoci ls --layout oci
    umoci unpack --image oci:img dest/img
    [ "$(sha reference/nm_orig)" == "$(sha .stacker/imports/img/nm)" ]
    [ "$(sha reference/nm_orig)" == "$(sha dest/img/rootfs/root/nm)" ]
}

@test "importing to a dest" {
    cat > img/stacker1.yaml <<EOF
centos_base:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - path: https://www.cisco.com/favicon.ico
          dest: /dest/icon
    run: |
        [ -f /dest/icon ]
        [ ! -f /dest/favicon.ico ]
        [ ! -f /stacker/favicon.ico ]
EOF
    # Build base image
    stacker build -f img/stacker1.yaml
    umoci ls --layout oci
}

# Ideally there would tests to hit/miss cache for servers which provide a hash
