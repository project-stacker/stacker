load helpers

function setup() {
    stacker_setup
}

function teardown() {
    rm -rf test-oci dest || true
    cleanup
}

@test "build only intermediate snapshots don't confuse things" {
    require_storage btrfs
    # get a linux to do stuff on
    # (double copy so we can take advantage of caching)
    mkdir -p .stacker/layer-bases
    skopeo --insecure-policy copy docker://centos:latest oci:.stacker/layer-bases/oci:centos
    skopeo --insecure-policy copy oci:.stacker/layer-bases/oci:centos oci:test-oci:a-linux

    cat > stacker.yaml <<EOF
# do some stuff
first:
    from:
        type: oci
        url: test-oci:a-linux
    run: |
        echo first
    build_only: true
second:
    from:
        type: oci
        url: test-oci:a-linux
    run: |
        echo second
    build_only: true
EOF
    stacker build

    # change the manifest so it is different, oh my!
    umoci config --image test-oci:a-linux --config.workingdir /usr
    umoci --log info gc --layout test-oci

    # now try it a second time...
    cat > stacker.yaml <<EOF
third:
    from:
        type: oci
        url: test-oci:a-linux
    run: |
        echo third
EOF
    stacker build
}

function test_intermediate_layers {
    layer_type=$1

    # as of this writing, the way the ubuntu image is generated it always has
    # ~4 layers, although they are small. below we fail the test if there are
    # not more than one layers, so that we can be sure the test always keeps
    # testing things.
    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://ubuntu:latest

EOF
    stacker build --leave-unladen --layer-type=$layer_type

    manifest=$(cat .stacker/layer-bases/oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    [ "$(cat .stacker/layer-bases/oci/blobs/sha256/$manifest | jq -r '.layers | length')" -gt "1" ]
    for i in $(seq 1 $(($(cat .stacker/layer-bases/oci/blobs/sha256/$manifest | jq -r '.layers | length')-1))); do
        accum=""
        for j in $(seq 0 $(($i-1))); do
            accum="$accum$(cat .stacker/layer-bases/oci/blobs/sha256/$manifest | jq -r ".layers[$j].digest")"
        done
        hash=$(echo -n "$accum" | sha256sum | cut -f1 -d" ")
        [ -d "roots/$hash" ]
    done
}

@test "intermediate base layers are snapshotted" {
    require_storage btrfs
    test_intermediate_layers tar
}

@test "intermediate base layers are snapshotted (squashfs)" {
    require_storage btrfs
    test_intermediate_layers squashfs
}

function test_intermediate_layers_used {
    layer_type=$1
    squashfs_suffix=
    if [ "$layer_type" == "squashfs" ]; then
        squashfs_suffix=-squashfs
    fi

    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $2

EOF
    stacker build --leave-unladen --layer-type=$layer_type
    manifest=$(cat oci-import/index.json | jq -r .manifests[0].digest | cut -f2 -d:)

    # Let's do some manual surgery to the base layer. Then we see if our manual
    # surgery persists. If stacker for whatever reason doesn't use the cached
    # version, our surgery won't be there.
    accum=""
    for i in $(seq 0 $(($(cat oci-import/blobs/sha256/$manifest | jq -r '.layers | length')-1))); do
        accum="$accum$(cat oci-import/blobs/sha256/$manifest | jq -r ".layers[$i].digest")"
    done
    lastlayer_hash=$(echo -n "$accum" | sha256sum | cut -f1 -d" ")
    btrfs property set -ts "roots/$lastlayer_hash" ro false
    touch "roots/$lastlayer_hash/rootfs/surgery"
    btrfs property set -ts "roots/$lastlayer_hash" ro true

    # now delete the old cached copy to force it to rebuild
    umoci rm --image oci:test$squashfs_suffix
    umoci gc --layout oci
    btrfs property set -ts "roots/test" ro false
    btrfs subvolume delete "roots/test"

    stacker build --leave-unladen --layer-type=$layer_type
    [ -f roots/test/rootfs/surgery ]
}

@test "intermediate base layers are used" {
    require_storage btrfs
    skopeo --insecure-policy copy docker://ubuntu:latest oci:oci-import:ubuntu
    test_intermediate_layers_used tar oci-import:ubuntu
}

@test "intermediate base layers are used (squashfs)" {
    require_storage btrfs
    cat > stacker.yaml <<EOF
ubuntu:
    from:
        type: docker
        url: docker://ubuntu:latest
    run:
        touch /foo
EOF
    stacker build --layer-type=squashfs
    mv oci oci-import
    stacker clean --all
    test_intermediate_layers_used squashfs oci-import:ubuntu-squashfs
}

function test_startfrom_respected {
    layer_type=$1
    squashfs_suffix=
    if [ "$layer_type" == "squashfs" ]; then
        squashfs_suffix=-squashfs
    fi
    cat > stacker.yaml <<EOF
ubuntu:
    from:
        type: docker
        url: docker://ubuntu:latest
    run:
        touch /foo
EOF
    stacker build --layer-type=$1
    mv oci oci-import
    stacker clean --all

    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: oci-import:ubuntu$squashfs_suffix

EOF
    stacker build --leave-unladen --layer-type=$layer_type
    manifest=$(cat oci-import/index.json | jq -r .manifests[0].digest | cut -f2 -d:)

    accum=""
    for i in $(seq 0 $(($(cat oci-import/blobs/sha256/$manifest | jq -r '.layers | length')-1))); do
        accum="$accum$(cat oci-import/blobs/sha256/$manifest | jq -r ".layers[$i].digest")"
    done
    penultimate_hash=$(echo -n "$accum" | sha256sum | cut -f1 -d" ")
    echo penultimate "$penultimate_hash"

    # delete the second to last layer, testing the startFrom extraction code
    btrfs property set -ts "roots/$penultimate_hash" ro false
    btrfs subvolume delete "roots/$penultimate_hash"

    umoci rm --image oci:test$squashfs_suffix
    umoci gc --layout oci
    btrfs property set -ts "roots/test" ro false
    btrfs subvolume delete "roots/test"

    stacker build --leave-unladen --layer-type=$layer_type
    ls roots/test/rootfs
    [ -f roots/test/rootfs/foo ]
}

@test "startFrom is respected" {
    require_storage btrfs
    test_startfrom_respected tar
}

@test "startFrom is respected (squashfs)" {
    require_storage btrfs
    test_startfrom_respected squashfs
}

@test "everything that gets umoci.json gets foo.mtree as well" {
    require_storage btrfs
    cat > stacker.yaml <<EOF
t1:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch t1

t2:
    from:
        type: oci
        url: ./oci:t1

    run: |
        touch t2

t3:
    from:
        type: oci
        url: ./oci:t2
    run: |
        touch t3

EOF
    stacker build
    umoci unpack --image oci:t3 dest

	[ -f dest/rootfs/t1 ]
	[ -f dest/rootfs/t2 ]
	[ -f dest/rootfs/t3 ]
}

@test "unprivileged intermediate base snapshot mtree generation" {
    require_storage btrfs
    [ -z "$TRAVIS" ] || skip "skipping unprivileged test in travis"

    cat > stacker.yaml <<EOF
parent:
    from:
        type: docker
        url: docker://centos:latest
    run: |
        touch /000
        chmod 000 /000
child:
    from:
        type: built
        tag: parent
    run: |
        touch /child
EOF

    # first build a base image
    stacker build
    mv oci oci-import
    stacker clean --all

    stacker unpriv-setup
    # now import that image twice, first the child image, then the parent
    # image, to force a layer regeneration
    cat > stacker.yaml <<'EOF'
child-child:
    from:
        type: oci
        url: oci-import:child
    run: |
        ls /
        stat --format="%a" /000
        [ "$(stat --format="%a" /000)" = "0" ]
        [ -f /child ]
parent-child:
    from:
        type: oci
        url: oci-import:parent
    run: |
        ls /
        [ "$(stat --format="%a" /000)" = "0" ]
        [ ! -f /child ]
        touch /foo
EOF
    chown -R $SUDO_USER:$SUDO_USER .
    sudo -u $SUDO_USER "${ROOT_DIR}/stacker" build

    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    n_layers=$(cat oci/blobs/sha256/$manifest | jq -r '.layers | length')
    last_layer=$(cat oci/blobs/sha256/$manifest | jq -r ".layers[$(($n_layers-1))].digest" | cut -f2 -d:)

    mkdir foo
    tar -C foo -xf oci/blobs/sha256/$last_layer
    ls -1 foo
    [ "$(ls -1 foo | wc -l)" = "1" ]
    # little bunny
    [ -f foo/foo ]
}
