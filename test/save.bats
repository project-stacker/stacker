load helpers

function setup() {
    cleanup
    rm -rf ocibuilds || true
    rm -rf oci_save || true
    rm -rf /tmp/ocibuilds || true
    mkdir -p ocibuilds/sub1
    mkdir oci_save
    touch ocibuilds/sub1/import1
    cat > ocibuilds/sub1/stacker.yaml <<EOF
layer1:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - import1
    run: |
        cp /stacker/import1 /root/import1
EOF
    mkdir -p ocibuilds/sub2
    touch ocibuilds/sub2/import2
    cat > ocibuilds/sub2/stacker.yaml <<EOF
stacker_config:
    prerequisites:
        - ../sub1/stacker.yaml
    save_url: oci:oci_save
layer2:
    from:
        type: built
        tag: layer1
    import:
        - import2
    run: |
        cp /stacker/import2 /root/import2
        cp /root/import1 /root/import1_copied
EOF
    mkdir -p ocibuilds/sub3
    cat > ocibuilds/sub3/stacker.yaml <<EOF
stacker_config:
    prerequisites:
        - ../sub2/stacker.yaml
    save_url: oci:oci_save
layer3:
    from:
        type: built
        tag: layer2
    run: |
        cp /root/import2 /root/import2_copied
EOF
    mkdir -p /tmp/ocibuilds/sub4
    cat > /tmp/ocibuilds/sub4/stacker.yaml <<EOF
stacker_config:
    save_url: oci:oci_save
layer4:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        ls > /root/ls_out
EOF
}

function teardown() {
    cleanup
    rm -rf ocibuilds || true
    rm -rf /tmp/ocibuilds || true
    rm -rf oci_save || true
}

@test "build layers and save them" {
    stacker build -f ocibuilds/sub3/stacker.yaml

    # Determine expected commit hash
    commit_hash=commit-$(git rev-parse --short HEAD)
    echo ${commit_hash}

    if [[ -z $(git status --porcelain --untracked-files=no) ]]; then
        # Unpack saved image and check content
        mkdir dest
        umoci unpack --image oci_save:layer2_${commit_hash} dest/layer2
        [ -f dest/layer2/rootfs/root/import2 ]
        [ -f dest/layer2/rootfs/root/import1_copied ]
        [ -f dest/layer2/rootfs/root/import1 ]
        umoci unpack --image oci_save:layer3_${commit_hash} dest/layer3
        [ -f dest/layer3/rootfs/root/import2_copied ]
        [ -f dest/layer3/rootfs/root/import2 ]
        [ -f dest/layer3/rootfs/root/import1_copied ]
        [ -f dest/layer3/rootfs/root/import1 ]
    else
        [[ "${output}" =~ ^(.*can\'t save layer layer2 since list of tags is empty)$ ]]
        [[ "${output}" =~ ^(.*can\'t save layer layer3 since list of tags is empty)$ ]]
    fi
}

@test "build layer and save it with custom tags" {
    stacker build -f ocibuilds/sub2/stacker.yaml --remote-save-tag test1 --remote-save-tag test2

    # Determine expected commit hash
    commit_hash=commit-$(git rev-parse --short HEAD)
    echo ${commit_hash}

    # Unpack saved image and check content
    mkdir dest
    if [[ -z $(git status --porcelain --untracked-files=no) ]]; then
        umoci unpack --image oci_save:layer2_${commit_hash} dest/layer2_commit
        [ -f dest/layer2_commit/rootfs/root/import2 ]
        [ -f dest/layer2_commit/rootfs/root/import1_copied ]
        [ -f dest/layer2_commit/rootfs/root/import1 ]
    else
        saved=$(umoci list --layout oci_save)
        [[ ! "${saved}" =~ ^(.*commit-.*)$ ]]
    fi
    umoci unpack --image oci_save:layer2_test1 dest/layer2_test1
    [ -f dest/layer2_test1/rootfs/root/import2 ]
    [ -f dest/layer2_test1/rootfs/root/import1_copied ]
    [ -f dest/layer2_test1/rootfs/root/import1 ]
    umoci unpack --image oci_save:layer2_test2 dest/layer2_test2
    [ -f dest/layer2_test2/rootfs/root/import2 ]
    [ -f dest/layer2_test2/rootfs/root/import1_copied ]
    [ -f dest/layer2_test2/rootfs/root/import1 ]
}

@test "build layer and save it without git tags" {
    stacker build -f /tmp/ocibuilds/sub4/stacker.yaml --remote-save-tag test1

    # Check absence of layer with commit in name
    saved=$(umoci list --layout oci_save)
    [[ ! "${saved}" =~ ^(.*commit-.*)$ ]]

    # Unpack saved image and check content
    mkdir dest
    umoci unpack --image oci_save:layer4_test1 dest/layer4_test1
    [ -f dest/layer4_test1/rootfs/root/ls_out ]
}