load helpers

function setup() {
    stacker_setup
}

function teardown() {
    rm -rf test-oci dest || true
    cleanup
}

@test "build only intermediate snapshots don't confuse things" {
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
    test_intermediate_layers tar
}

@test "intermediate base layers are snapshotted (squashfs)" {
    test_intermediate_layers squashfs
}

function test_intermediate_layers_used {
    layer_type=$1

    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: oci-import:ubuntu

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
    umoci rm --image oci:test
    umoci gc --layout oci
    btrfs property set -ts "roots/test" ro false
    btrfs subvolume delete "roots/test"

    stacker build --leave-unladen --layer-type=$layer_type
    [ -f roots/test/rootfs/surgery ]
}

@test "intermediate base layers are used" {
    skopeo --insecure-policy copy docker://ubuntu:latest oci:oci-import:ubuntu
    test_intermediate_layers_used tar oci-import:ubuntu
}

@test "intermediate base layers are used (squashfs)" {
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
    test_intermediate_layers_used squashfs oci-import:ubuntu
}

@test "everything that gets umoci.json gets foo.mtree as well" {
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
