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
config:
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
config:
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
}

function teardown() {
    cleanup
    rm -rf ocibuilds || true
}

@test "order prerequisites" {
    # Search for sub3/stacker.yaml and produce a build order including all other stackerfiles it depends on
    stacker build -f ocibuilds/sub3/stacker.yaml --order-only
    [[ "${lines[-1]}" =~ ^(2 build .*ocibuilds\/sub3\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml .*\/sub2\/stacker\.yaml\])$ ]]
    [[ "${lines[-2]}" =~ ^(1 build .*ocibuilds\/sub2\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml\])$ ]]
    [[ "${lines[-3]}" =~ ^(0 build .*ocibuilds\/sub1\/stacker\.yaml: requires: \[\])$ ]]
}

@test "search for multiple stackerfiles using default settings" {
    cd ocibuilds
    # Search for all stackerfiles under current directory and produce a build order
    stacker recursive-build --order-only
    [[ "${lines[-1]}" =~ ^(2 build .*ocibuilds\/sub3\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml .*\/sub2\/stacker\.yaml\])$ ]]
    [[ "${lines[-2]}" =~ ^(1 build .*ocibuilds\/sub2\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml\])$ ]]
    [[ "${lines[-3]}" =~ ^(0 build .*ocibuilds\/sub1\/stacker\.yaml: requires: \[\])$ ]]
}

@test "search for multiple stackerfiles using a custom search directory" {
    # Search for all stackerfiles under ocibuilds and produce a build order
    stacker recursive-build --search-dir ocibuilds --order-only
    [[ "${lines[-1]}" =~ ^(2 build .*ocibuilds\/sub3\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml .*\/sub2\/stacker\.yaml\])$ ]]
    [[ "${lines[-2]}" =~ ^(1 build .*ocibuilds\/sub2\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml\])$ ]]
    [[ "${lines[-3]}" =~ ^(0 build .*ocibuilds\/sub1\/stacker\.yaml: requires: \[\])$ ]]
}

@test "search for multiple stackerfiles using a custom search directory and a custom file path pattern" {
    stacker recursive-build --search-dir ocibuilds -p sub2/stacker.yaml --order-only
    # Only sub2/stacker.yaml should be found
    # since sub2/stacker.yaml depends on sub1/stacker.yaml, sub1/stacker.yaml will also be rebuilt
    # since sub3/stacker.yaml is filtered out and sub2.stacker.yaml does not depend on it, it can be ignored
    [[ "${lines[-1]}" =~ ^(1 build .*ocibuilds\/sub2\/stacker\.yaml: requires: \[.*\/sub1\/stacker\.yaml\])$ ]]
    [[ "${lines[-2]}" =~ ^(0 build .*ocibuilds\/sub1\/stacker\.yaml: requires: \[\])$ ]]
}

@test "build layers and prerequisites using the build command" {
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

@test "build layers and prerequisites using the recursive-build command" {
    stacker recursive-build -d ocibuilds
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

@test "build layers and prerequisites containing build-only layer" {
    mkdir -p ocibuilds/sub4
    cat > ocibuilds/sub4/stacker.yaml <<EOF
config:
    prerequisites:
        - ../sub1/stacker.yaml
layer4_1:
    from:
        type: built
        tag: layer1_2
    run: |
        touch /root/import4
    build_only: true
layer4_2:
    from:
        type: built
        tag: layer4_1
    run: |
        cp /root/import4 /root/import4_copied
EOF
    stacker build -f ocibuilds/sub4/stacker.yaml
    mkdir dest
    umoci unpack --image oci:layer4_2 dest/layer4_2
    [ "$status" -eq 0 ]
    [ -f dest/layer4_2/rootfs/root/import4_copied ]
    [ -f dest/layer4_2/rootfs/root/import4 ]
    [ -f dest/layer4_2/rootfs/root/import0 ]
}