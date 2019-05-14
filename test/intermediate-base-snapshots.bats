load helpers

function teardown() {
    cleanup
}

@test "intermediate base layers are snapshotted" {
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
    stacker build --leave-unladen

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$manifest | jq -r '.layers | length')" -gt "1" ]
    for i in $(seq 1 $(($(cat oci/blobs/sha256/$manifest | jq -r '.layers | length')-1))); do
        accum=""
        for j in $(seq 0 $(($i-1))); do
            accum="$accum$(cat oci/blobs/sha256/$manifest | jq -r ".layers[$j].digest")"
        done
        hash=$(echo -n "$accum" | sha256sum | cut -f1 -d" ")
        [ -d "roots/$hash" ]
    done
}

@test "intermediate base layers are used" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: docker
        url: docker://ubuntu:latest

EOF
    stacker build --leave-unladen
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)

    # Let's do some manual surgery to the base layer. Then we see if our manual
    # surgery persists. If stacker for whatever reason doesn't use the cached
    # version, our surgery won't be there.
    accum=""
    for i in $(seq 0 $(($(cat oci/blobs/sha256/$manifest | jq -r '.layers | length')-1))); do
        accum="$accum$(cat oci/blobs/sha256/$manifest | jq -r ".layers[$i].digest")"
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
    btrfs property list -ts roots/_working

    stacker build --leave-unladen
    [ -f roots/test/rootfs/surgery ]
}
