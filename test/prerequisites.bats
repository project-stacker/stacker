load helpers

function setup() {
    cleanup
    rm -rf ocibuilds || true
    mkdir -p ocibuilds/sub1
    touch ocibuilds/sub1/import1
    cat > ocibuilds/sub1/stacker.yaml <<EOF
layer1_1:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - import1
    run: |
        cp /stacker/import1 /root/import1
layer1_2:
    from:
        type: docker
        url: docker://centos:latest
    run:
        touch /root/import0
EOF
    mkdir -p ocibuilds/sub2
    touch ocibuilds/sub2/import2
    cat > ocibuilds/sub2/stacker.yaml <<EOF
stacker_config:
    prerequisites:
        - ../sub1/stacker.yaml
layer2:
    from:
        type: built
        tag: layer1_1
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
        - ../sub1/stacker.yaml
        - ../sub2/stacker.yaml
layer3_1:
    from:
        type: built
        tag: layer2
    run: |
        cp /root/import2 /root/import2_copied
layer3_2:
    from:
        type: built
        tag: layer1_2
    run: |
        cp /root/import0 /root/import0_copied
EOF
    mkdir -p ocibuilds/sub4
    touch ocibuilds/sub4/import4
    cat > ocibuilds/sub4/stacker.yaml <<EOF
layer4:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - import4
    run: |
        cp /stacker/import4 /root/import4
EOF
    mkdir -p ocibuilds/sub5
    touch ocibuilds/sub5/import5
    cat > ocibuilds/sub5/stacker.yaml <<EOF
stacker_config:
    prerequisites:
        - ../sub4/stacker.yaml
layer5:
    from:
        type: built
        tag: layer4
    import:
        - import5
    run: |
        cp /stacker/import5 /root/import5
EOF
}

function teardown() {
    cleanup
    rm -rf ocibuilds || true
}

@test "order prerequisites" {
    stacker build -f ocibuilds/sub3/stacker.yaml --order-only
    [[ "${lines[-1]}" =~ ^(2 build .*ocibuilds\/sub3\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml .*\/sub2\/stacker\.yaml\])$ ]]
    [[ "${lines[-2]}" =~ ^(1 build .*ocibuilds\/sub2\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml\])$ ]]
    [[ "${lines[-3]}" =~ ^(0 build .*ocibuilds\/sub1\/stacker\.yaml: requires: \[\])$ ]]
}

@test "build layers and prerequisites for a single stackerfile" {
    stacker build -f ocibuilds/sub3/stacker.yaml
    mkdir dest
    umoci unpack --image oci:layer3_1 dest/layer3_1
    [ "$status" -eq 0 ]
    [ -f dest/layer3_1/rootfs/root/import2_copied ]
    [ -f dest/layer3_1/rootfs/root/import2 ]
    [ -f dest/layer3_1/rootfs/root/import1_copied ]
    [ -f dest/layer3_1/rootfs/root/import1 ]
    umoci unpack --image oci:layer3_2 dest/layer3_2
    [ "$status" -eq 0 ]
    [ -f dest/layer3_2/rootfs/root/import0_copied ]
    [ -f dest/layer3_2/rootfs/root/import0 ]
}

@test "build layers and prerequisites for multiple stackerfiles" {
    stacker build -f ocibuilds/sub5/stacker.yaml -f ocibuilds/sub3/stacker.yaml
    mkdir dest
    umoci unpack --image oci:layer3_1 dest/layer3_1
    [ "$status" -eq 0 ]
    [ -f dest/layer3_1/rootfs/root/import2_copied ]
    [ -f dest/layer3_1/rootfs/root/import2 ]
    [ -f dest/layer3_1/rootfs/root/import1_copied ]
    [ -f dest/layer3_1/rootfs/root/import1 ]
    umoci unpack --image oci:layer3_2 dest/layer3_2
    [ "$status" -eq 0 ]
    [ -f dest/layer3_2/rootfs/root/import0_copied ]
    [ -f dest/layer3_2/rootfs/root/import0 ]
    umoci unpack --image oci:layer5 dest/layer5
    [ "$status" -eq 0 ]
    [ -f dest/layer5/rootfs/root/import4 ]
    [ -f dest/layer5/rootfs/root/import5 ]
}
