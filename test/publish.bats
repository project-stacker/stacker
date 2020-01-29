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


@test "publish layer with custom tag" {
    stacker build -f /tmp/ocibuilds/sub4/stacker.yaml
    stacker publish -f /tmp/ocibuilds/sub4/stacker.yaml --url oci:oci_publish --tag test1

     # Unpack published image and check content
    mkdir dest
    umoci unpack --image oci_publish:layer4_test1 dest/layer4_test1
    [ -f dest/layer4_test1/rootfs/root/ls_out ]
}


@test "publish layer with multiple custom tags" {
    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish -f ocibuilds/sub1/stacker.yaml --url oci:oci_publish --tag test1 --tag test2

     # Unpack published image and check content
    mkdir dest
    umoci unpack --image oci_publish:layer1_test1 dest/layer1_test1
    [ -f dest/layer1_test1/rootfs/root/import1 ]
    umoci unpack --image oci_publish:layer1_test2 dest/layer1_test2
    [ -f dest/layer1_test2/rootfs/root/import1 ]
}


@test "publish multiple layers recursively" {
    stacker recursive-build -d ocibuilds
    stacker publish -d ocibuilds --url oci:oci_publish --tag test1

     # Unpack published image and check content
    umoci unpack --image oci_publish:layer1_test1 dest/layer1_test1
    [ -f dest/layer1_test1/rootfs/root/import1 ]
    umoci unpack --image oci_publish:layer2_test1 dest/layer2_test1
    [ -f dest/layer2_test1/rootfs/root/import2 ]
    [ -f dest/layer2_test1/rootfs/root/import1_copied ]
    [ -f dest/layer2_test1/rootfs/root/import1 ]
}

@test "publish single layer with docker url" {
    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish -f ocibuilds/sub1/stacker.yaml --url docker://docker-reg.fake.com/ --username user --password pass --tag test1 --show-only

    # Check command output
    [[ "${lines[-1]}" =~ ^(would publish: ocibuilds\/sub1\/stacker.yaml layer1 to docker://docker-reg.fake.com/layer1:test1)$ ]]

}

@test "do not publish build only layer" {
    stacker build -f /tmp/ocibuilds/sub3/stacker.yaml
    stacker publish -f /tmp/ocibuilds/sub3/stacker.yaml --url oci:oci_publish --tag test1
    # Check the output does not contain the tag since no images should be published
    [[ ${output} =~ "will not publish: /tmp/ocibuilds/sub3/stacker.yaml build_only layer3" ]]
}
