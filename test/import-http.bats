load helpers

function teardown() {
    cleanup
    rm -rf img || true
    rm -rf reference || true
    rm -rf dest || true
    sudo ip netns del stacker-test || true
}

function setup() {
    mkdir reference
    mkdir dest
    rm -f nm_orig
    wget http://network-test.debian.org/nm -O reference/nm_orig
    mkdir img
    sudo ip netns add stacker-test
    # Need to separate layer download from the download of the test file for imports
    # As we want to be able to have network for base image download
    # in img/stacker1.yaml but disconnect the network for test file download
    cat > img/stacker1.yaml <<EOF
centos_base:
    from:
        type: docker
        url: docker://centos:latest
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

@test "importing from cache works for unreachable http urls" {
    # Build base image
    stacker build -f img/stacker1.yaml
    umoci ls --layout oci
    # First execution creates the cache
    stacker build -f img/stacker2.yaml
    umoci ls --layout oci
    # Second execution reads from the cache, but cannot access the net
    run ip netns exec stacker-test "${ROOT_DIR}/stacker" build -f img/stacker2.yaml
    [ "$status" -eq 0 ]
    umoci ls --layout oci
    umoci unpack --image oci:img dest/img
    [ "$(sha reference/nm_orig)" == "$(sha .stacker/imports/img/nm)" ]
    [ "$(sha reference/nm_orig)" == "$(sha dest/img/rootfs/root/nm)" ]
}

@test "importing cached file from http url with matching lenght" {
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

# Ideally there would tests to hit/miss cache for servers which provide a hash
