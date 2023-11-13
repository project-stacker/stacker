load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm favicon.ico >& /dev/null || true
}

@test "build only + missing prereq fails" {
    cat > prereq.yaml <<EOF
parent:
    from:
        type: oci
        url: $BUSYBOX_OCI
EOF

    cat > stacker.yaml <<EOF
config:
    prerequisites:
        - ./prereq.yaml
child:
    from:
        type: built
        tag: zomg
    run: echo "d2" > /bestgame
EOF
    bad_stacker build
    echo $output | grep "couldn't resolve some dependencies"
}

@test "build only + prerequisites work" {
    cat > prereq.yaml <<EOF
parent:
    from:
        type: oci
        url: $BUSYBOX_OCI
EOF

    cat > stacker.yaml <<EOF
config:
    prerequisites:
        - ./prereq.yaml
child:
    from:
        type: built
        tag: parent
    run: echo "d2" > /bestgame
EOF
    stacker build
    umoci unpack --image oci:child dest
    [ "$(cat dest/rootfs/bestgame)" == "d2" ]
}

@test "after build only failure works" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        false
    build_only: true
child:
    from:
        type: built
        tag: parent
    run: |
        touch /child
EOF
    bad_stacker build
    sed 's/false/true/g' -i stacker.yaml
    stacker build
    umoci unpack --image oci:child dest
    [ -f dest/rootfs/child ]
}

@test "build only stacker" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    imports: https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/imports/favicon.ico /favicon.ico
    build_only: true
layer1:
    from:
        type: built
        tag: busybox
    imports:
        - stacker://busybox/favicon.ico
    run:
        - cp /stacker/imports/favicon.ico /favicon2.ico
EOF
    stacker build
    umoci unpack --image oci:layer1 dest
    [ "$(sha dest/rootfs/favicon.ico)" == "$(sha dest/rootfs/favicon2.ico)" ]
    [ "$(umoci ls --layout ./oci)" == "$(printf "layer1")" ]
}

@test "stacker grab" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    imports: https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/imports/favicon.ico /favicon.ico
    build_only: true
layer1:
    from:
        type: built
        tag: busybox
    imports:
        - stacker://busybox/favicon.ico
    run:
        - cp /stacker/imports/favicon.ico /favicon2.ico
EOF
    stacker build
    stacker grab busybox:/favicon.ico
    [ -f favicon.ico ]
    [ "$(sha favicon.ico)" == "$(sha .stacker/imports/busybox/favicon.ico)" ]
}

@test "build only + unpriv + overlay clears state" {
    cat > stacker.yaml <<"EOF"
first:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    build_only: true
    run: |
        echo "run number ${{RUN_NUMBER}}"
        THEPATH=/root/dir
        [ ! -d $THEPATH ]
        mkdir -p $THEPATH

        # make it readonly to host stacker
        chmod 500 $THEPATH
EOF

    stacker build --layer-type=squashfs --substitute "RUN_NUMBER=1" --substitute BUSYBOX_OCI=$BUSYBOX_OCI
    stacker build --layer-type=squashfs --substitute "RUN_NUMBER=2" --substitute BUSYBOX_OCI=$BUSYBOX_OCI
}

@test "multiple build onlys in final chain rebuild OK" {
    cat > stacker.yaml <<"EOF"
one:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        touch /1
two:
    from:
        type: built
        tag: one
    run: |
        touch /2
    build_only: true
three:
    from:
        type: built
        tag: two
    run: |
        touch /3
    build_only: true
four:
    from:
        type: built
        tag: three
    run: |
        ls /
        [ -f /1 ]
        [ -f /2 ]
        [ -f /3 ]
        echo run number ${{RUN_NUMBER}}
EOF
    stacker build --substitute "RUN_NUMBER=1" --substitute BUSYBOX_OCI=$BUSYBOX_OCI
    stacker build --substitute "RUN_NUMBER=2" --substitute BUSYBOX_OCI=$BUSYBOX_OCI

    stacker inspect

    # should always build five layers, assuming busybox is 1 layer
    one_manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    one_layercount=$(cat oci/blobs/sha256/$one_manifest | jq -r '.layers | length')
    echo one_layercount "$one_layercount"
    [ "$one_layercount" = 2 ]

    four_manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    four_layercount=$(cat oci/blobs/sha256/$four_manifest | jq -r '.layers | length')
    echo four_layercount "$four_layercount"
    [ "$four_layercount" = 5 ]

    cat oci/blobs/sha256/$four_manifest | jq

    # we should be able to extract this thing too...
    umoci unpack --image oci:four dest
    [ -f dest/rootfs/1 ]
    [ -f dest/rootfs/2 ]
    [ -f dest/rootfs/3 ]
}
