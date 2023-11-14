load helpers

function setup() {
    stacker_setup
    mkdir -p ocibuilds/sub1
    touch ocibuilds/sub1/import1
    cat > ocibuilds/sub1/stacker.yaml <<EOF
layer1:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - import1
    run: |
        cp /stacker/imports/import1 /root/import1
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
        cp /stacker/imports/import2 /root/import2
        cp /root/import1 /root/import1_copied
EOF
    mkdir -p ocibuilds/sub3
    cat > ocibuilds/sub3/stacker.yaml <<EOF
layer3:
    from:
        type: oci
        url: $BUSYBOX_OCI
    build_only: true
EOF
    mkdir -p ocibuilds/sub4
    cat > ocibuilds/sub4/stacker.yaml <<EOF
layer4:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        ls > /root/ls_out
EOF
    mkdir -p ocibuilds/sub5
    cat > ocibuilds/sub5/stacker.yaml <<EOF
config:
    prerequisites:
        - ../sub1/stacker.yaml
layer5:
    from:
        type: built
        tag: layer1
    build_only: true
EOF
    mkdir -p ocibuilds/sub6
    cat > ocibuilds/sub6/stacker.yaml <<EOF
config:
    prerequisites:
        - ../sub5/stacker.yaml
layer6:
    from:
        type: built
        tag: layer5
    run: |
        ls > /root/ls_out
EOF
}

function teardown() {
    cleanup
    rm -rf ocibuilds || true
    rm -rf oci_publish || true
}


@test "publish layer with custom tag" {
    stacker build -f ocibuilds/sub4/stacker.yaml
    stacker publish -f ocibuilds/sub4/stacker.yaml --url oci:oci_publish --tag test1

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

@test "publish selected multiple layers" {
    stacker recursive-build -d ocibuilds
    stacker publish -d ocibuilds --url oci:oci_publish --tag test1 --layer layer1 --layer layer6

     # Unpack published image and check content
    umoci unpack --image oci_publish:layer1_test1 dest/layer1_test1
    [ -f dest/layer1_test1/rootfs/root/import1 ]
    umoci unpack --image oci_publish:layer6_test1 dest/layer6_test1
    [ -f dest/layer6_test1/rootfs/root/ls_out ]
    # since we did not publish this layer, shouldn't be found
    run umoci unpack --image oci_publish:layer2_test1 dest/layer2_test1
    [ "$status" -ne 0 ]
}

@test "publish single layer with docker url" {
    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish -f ocibuilds/sub1/stacker.yaml --url docker://docker-reg.fake.com/ --username user --password pass --tag test1 --show-only

    # Check command output
    [[ "${lines[-1]}" =~ ^(would publish: ocibuilds\/sub1\/stacker.yaml layer1 to docker://docker-reg.fake.com/layer1:test1)$ ]]

}

@test "do not publish build only layer" {
    stacker build -f ocibuilds/sub3/stacker.yaml
    stacker publish -f ocibuilds/sub3/stacker.yaml --url oci:oci_publish --tag test1
    # Check the output does not contain the tag since no images should be published
    [[ ${output} =~ "will not publish: ocibuilds/sub3/stacker.yaml build_only layer3" ]]
}

@test "publish multiple layer types" {
    stacker --storage-type overlay build -f ocibuilds/sub4/stacker.yaml --layer-type tar --layer-type squashfs
    stacker --storage-type overlay publish -f ocibuilds/sub4/stacker.yaml --layer-type tar --layer-type squashfs --url oci:oci_publish --tag test1

    mkdir dest

    # check that we have the right tar output
    umoci unpack --image oci_publish:layer4_test1 dest/layer4_test1
    [ -f dest/layer4_test1/rootfs/root/ls_out ]

    # and the squashfs output?
    umoci ls --layout oci_publish | grep layer4_test1-squashfs
    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)
    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ -f layer1/root/ls_out ]
}

@test "publish tag to unsecure registry" {
    if [ -z "${REGISTRY_URL}" ]; then
        skip "skipping test because no registry found in REGISTRY_URL env variable"
    fi

    stacker build -f ocibuilds/sub4/stacker.yaml
    stacker publish --skip-tls -f ocibuilds/sub4/stacker.yaml --url docker://${REGISTRY_URL} --tag test1

    # check content of published image
    # should have /root/ls_out from sub4/stacker.yaml
    mkdir -p ocibuilds/sub7
    cat > ocibuilds/sub7/stacker.yaml <<EOF
published:
    from:
        type: docker
        url: docker://${REGISTRY_URL}/layer4:test1
    run: |
        cat /root/ls_out
EOF

}

@test "publish multiple tags to unsecure registry" {
    if [ -z "${REGISTRY_URL}" ]; then
        skip "skipping test because no registry found in REGISTRY_URL env variable"
    fi

    stacker build -f ocibuilds/sub1/stacker.yaml
    stacker publish --skip-tls -f ocibuilds/sub1/stacker.yaml --url docker://${REGISTRY_URL} --tag test1 --tag test2

    # check content of published image
    # should have /root/import1 from sub1/stacker.yaml
    mkdir -p ocibuilds/sub8
    cat > ocibuilds/sub8/stacker.yaml <<EOF
published:
    from:
        type: docker
        url: docker://${REGISTRY_URL}/layer1:test1
    run: |
        [ -f /root/import1 ]
EOF

    # check content of published image
    # should have /root/import1 from sub1/stacker.yaml
    mkdir -p ocibuilds/sub9
    cat > ocibuilds/sub9/stacker.yaml <<EOF
published:
    from:
        type: docker
        url: docker://${REGISTRY_URL}/layer1:test2
    run: |
        [ -f /root/import1 ]
EOF


}

@test "publish multiple layers recursively to unsecure registry" {
    if [ -z "${REGISTRY_URL}" ]; then
        skip "skipping test because no registry found in REGISTRY_URL env variable"
    fi

    stacker recursive-build -d ocibuilds
    stacker publish --skip-tls -d ocibuilds --url docker://${REGISTRY_URL} --tag test1

    # check content of published image
    # layer6 should have /root/import1 from sub1/stacker.yaml
    mkdir -p ocibuilds/sub10
    cat > ocibuilds/sub10/stacker.yaml <<EOF
published:
    from:
        type: docker
        url: docker://${REGISTRY_URL}/layer6:test1
    run: |
        [ -f /root/import1 ]
EOF

}
