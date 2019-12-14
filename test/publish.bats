load helpers

function setup() {
    cleanup
    rm -rf ocibuilds || true
    rm -rf oci_publish || true
    rm -rf /tmp/ocibuilds || true
    mkdir -p ocibuilds/sub1
    mkdir oci_publish
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
config:
    prerequisites:
        - ../sub1/stacker.yaml
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
    mkdir -p /tmp/ocibuilds/sub3
    cat > /tmp/ocibuilds/sub3/stacker.yaml <<EOF
layer3:
    from:
        type: scratch
    build_only: true
EOF
    mkdir -p /tmp/ocibuilds/sub4
    cat > /tmp/ocibuilds/sub4/stacker.yaml <<EOF
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
    rm -rf oci_publish || true
}

@test "publish layer with git tag only" {
    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish -f ocibuilds/sub1/stacker.yaml --url oci:oci_publish

     # Determine expected commit hash
    commit_hash=commit-$(git rev-parse --short HEAD)
    echo ${commit_hash}

     # Unpack published image and check content
    mkdir dest
    if [[ -z $(git status --porcelain --untracked-files=no) ]]; then
        # The repo doesn't have local changes so the commit_hash tag is applied
        umoci unpack --image oci_publish:layer1_${commit_hash} dest/layer1_commit
        [ -f dest/layer1_commit/rootfs/root/import1 ]
    else
        # The repo has local changes, don't apply the commit_hash
        [[ ${output} =~ "since list of tags is empty" ]]
    fi
}

@test "publish layer with git and custom tags" {
    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish -f ocibuilds/sub1/stacker.yaml --url oci:oci_publish --tag test1 --tag test2

     # Determine expected commit hash
    commit_hash=commit-$(git rev-parse --short HEAD)
    echo ${commit_hash}

     # Unpack published image and check content
    mkdir dest
    if [[ -z $(git status --porcelain --untracked-files=no) ]]; then
        # The repo doesn't have local changes so the commit_hash tag is applied
        umoci unpack --image oci_publish:layer1_${commit_hash} dest/layer1_commit
        [ -f dest/layer1_commit/rootfs/root/import1 ]
    else
        # The repo has local changes, don't apply the commit_hash
        published=$(umoci list --layout oci_publish)
        [[ ! "${published}" =~ ^(.*commit-.*)$ ]]
    fi
    umoci unpack --image oci_publish:layer1_test1 dest/layer1_test1
    [ -f dest/layer1_test1/rootfs/root/import1 ]
    umoci unpack --image oci_publish:layer1_test2 dest/layer1_test2
    [ -f dest/layer1_test2/rootfs/root/import1 ]
}

@test "publish layer with custom tag but without git tag" {
    stacker build -f /tmp/ocibuilds/sub4/stacker.yaml
    stacker publish -f /tmp/ocibuilds/sub4/stacker.yaml --url oci:oci_publish --tag test1

     # Check absence of layer with commit in name
    published=$(umoci list --layout oci_publish)
    [[ ! "${published}" =~ ^(.*commit-.*)$ ]]

     # Unpack published image and check content
    mkdir dest
    umoci unpack --image oci_publish:layer4_test1 dest/layer4_test1
    [ -f dest/layer4_test1/rootfs/root/ls_out ]
}

@test "publish multiple layers recursively" {
    stacker recursive-build -d ocibuilds
    stacker publish -d ocibuilds --url oci:oci_publish --tag test1

     # Determine expected commit hash
    commit_hash=commit-$(git rev-parse --short HEAD)
    echo ${commit_hash}

     # Unpack published image and check content
    mkdir dest
    if [[ -z $(git status --porcelain --untracked-files=no) ]]; then
        umoci unpack --image oci_publish:layer1_${commit_hash} dest/layer1_commit
        [ -f dest/layer1_commit/rootfs/root/import1 ]
        umoci unpack --image oci_publish:layer2_${commit_hash} dest/layer2_commit
        [ -f dest/layer2_commit/rootfs/root/import2 ]
        [ -f dest/layer2_commit/rootfs/root/import1_copied ]
        [ -f dest/layer2_commit/rootfs/root/import1 ]
    else
        published=$(umoci list --layout oci_publish)
        [[ ! "${published}" =~ ^(.*commit-.*)$ ]]
    fi
    umoci unpack --image oci_publish:layer1_test1 dest/layer1_test1
    [ -f dest/layer1_test1/rootfs/root/import1 ]
    umoci unpack --image oci_publish:layer2_test1 dest/layer2_test1
    [ -f dest/layer2_test1/rootfs/root/import2 ]
    [ -f dest/layer2_test1/rootfs/root/import1_copied ]
    [ -f dest/layer2_test1/rootfs/root/import1 ]
}

@test "publish single layer with docker url" {

    # Determine expected commit hash
    commit_hash=commit-$(git rev-parse --short HEAD)
    untracked=$(git status --porcelain --untracked-files=no)
    echo ${commit_hash}
    echo ${untracked}

    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish -f ocibuilds/sub1/stacker.yaml --url docker://docker-reg.fake.com/ --username user --password pass --tag test1 --show-only

    # Check command output
    if [[ -z ${untracked} ]]; then
        # The repo doesn't have local changes so the commit_hash tag is applied
        [[ "${lines[-2]}" =~ ^(would publish: ocibuilds\/sub1\/stacker.yaml layer1 to docker://docker-reg.fake.com/layer1:test1)$ ]]
        [[ "${lines[-1]}" =~ ^(would publish: ocibuilds\/sub1\/stacker.yaml layer1 to docker://docker-reg.fake.com/layer1:commit-.*)$ ]]

    else
        # The repo has local changes so the commit_hash tag is not applied
        [[ "${lines[-1]}" =~ ^(would publish: ocibuilds\/sub1\/stacker.yaml layer1 to docker://docker-reg.fake.com/layer1:test1)$ ]]
    fi

}

@test "do not publish build only layer" {
    stacker build -f /tmp/ocibuilds/sub3/stacker.yaml
    stacker publish -f /tmp/ocibuilds/sub3/stacker.yaml --url oci:oci_publish --tag test1
    # Check the output does not contain the tag since no images should be published
    [[ ${output} =~ "will not publish: /tmp/ocibuilds/sub3/stacker.yaml build_only layer3" ]]
}
