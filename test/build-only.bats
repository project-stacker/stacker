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
        url: $CENTOS_OCI
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
        url: $CENTOS_OCI
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
        url: $CENTOS_OCI
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
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import: https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
    build_only: true
layer1:
    from:
        type: built
        tag: centos
    import:
        - stacker://centos/favicon.ico
    run:
        - cp /stacker/favicon.ico /favicon2.ico
EOF
    stacker build
    umoci unpack --image oci:layer1 dest
    [ "$(sha dest/rootfs/favicon.ico)" == "$(sha dest/rootfs/favicon2.ico)" ]
    [ "$(umoci ls --layout ./oci)" == "$(printf "layer1")" ]
}

@test "stacker grab" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import: https://www.cisco.com/favicon.ico
    run: |
        cp /stacker/favicon.ico /favicon.ico
    build_only: true
layer1:
    from:
        type: built
        tag: centos
    import:
        - stacker://centos/favicon.ico
    run:
        - cp /stacker/favicon.ico /favicon2.ico
EOF
    stacker build
    stacker grab centos:/favicon.ico
    [ -f favicon.ico ]
    [ "$(sha favicon.ico)" == "$(sha .stacker/imports/centos/favicon.ico)" ]
}

@test "build only + unpriv + overlay clears state" {
    require_storage overlay
    cat > stacker.yaml <<"EOF"
first:
    from:
        type: oci
        url: ${{CENTOS_OCI}}
    build_only: true
    run: |
        echo "run number ${{RUN_NUMBER}}"
        THEPATH=/root/dir
        [ ! -d $THEPATH ]
        mkdir -p $THEPATH

        # make it readonly to host stacker
        chmod 500 $THEPATH
EOF

    stacker build --layer-type=squashfs --substitute "RUN_NUMBER=1" --substitute CENTOS_OCI=$CENTOS_OCI
    stacker build --layer-type=squashfs --substitute "RUN_NUMBER=2" --substitute CENTOS_OCI=$CENTOS_OCI
}
