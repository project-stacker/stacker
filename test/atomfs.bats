load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

function basic_test() {
    require_privilege priv
    local verity_arg=$1

    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /hello
EOF
    stacker build --layer-type=squashfs $verity_arg
    mkdir mountpoint
    stacker internal-go atomfs mount test-squashfs mountpoint

    [ -f mountpoint/hello ]
    stacker internal-go atomfs umount mountpoint
}

@test "--no-squashfs-verity works" {
    basic_test --no-squashfs-verity
}

@test "mount + umount works" {
    basic_test

    # last layer shouldn't exist any more, since it is unique
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    last_layer_num=$(($(cat oci/blobs/sha256/$manifest | jq -r '.layers | length')-1))
    last_layer_hash=$(cat oci/blobs/sha256/$manifest | jq -r .layers[$last_layer].digest | cut -f2 -d:)
    [ ! -b "/dev/mapper/$last_layer_hash-verity" ]
}

@test "mount + umount + mount a tree of images works" {
    skip "this branch doesn't have full support"
    require_privilege priv
    cat > stacker.yaml <<EOF
base:
    from:
        type: oci
        url: $CENTOS_OCI
    run: touch /base
a:
    from:
        type: built
        tag: base
    run: touch /a
b:
    from:
        type: built
        tag: base
    run: touch /b
c:
    from:
        type: built
        tag: base
    run: touch /c
EOF
    stacker build --layer-type=squashfs

    mkdir a
    stacker internal-go atomfs mount a-squashfs a
    [ -f a/a ]

    mkdir b
    stacker internal-go atomfs mount b-squashfs b
    [ -f b/b ]

    cat /proc/self/mountinfo
    echo "mountinfo after b^"

    stacker internal-go atomfs umount b

    # first layer should still exist since a is still mounted
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    first_layer_hash=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    [ ! -b "/dev/mapper/$last_layer_hash-verity" ]

    mkdir c
    stacker internal-go atomfs mount c-squashfs c
    [ -f c/c ]

    cat /proc/self/mountinfo
    echo "mountinfo after c^"

    stacker internal-go atomfs umount a

    cat /proc/self/mountinfo
    echo "mountinfo after umount a^"

    # first layer should still exist since c is still mounted
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    first_layer_hash=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    [ ! -b "/dev/mapper/$last_layer_hash-verity" ]

    # c should still be ok
    [ -f c/c ]
    [ -f c/sbin/init ]
    stacker internal-go atomfs umount c

    # c's last layer shouldn't exist any more, since it is unique
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    last_layer_num=$(($(cat oci/blobs/sha256/$manifest | jq -r '.layers | length')-1))
    last_layer_hash=$(cat oci/blobs/sha256/$manifest | jq -r .layers[$last_layer].digest | cut -f2 -d:)
    [ ! -b "/dev/mapper/$last_layer_hash-verity" ]
}

@test "bad existing verity device is rejected" {
    require_privilege priv
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /hello
EOF
    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    first_layer_hash=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    devname="$first_layer_hash-verity"

    # make an evil device and fake it as an existing verity device
    dd if=/dev/random of=mydev bs=50K count=1
    root_hash=$(veritysetup format mydev mydev.hash | grep "Root hash:" | awk '{print $NF}')
    echo "root hash $root_hash"
    veritysetup open mydev "$devname" mydev.hash "$root_hash"

    mkdir mountpoint
    bad_stacker internal-go atomfs mount test-squashfs mountpoint | grep "invalid root hash"
    veritysetup close "$devname"
}
