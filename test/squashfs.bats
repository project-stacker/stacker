load helpers

function setup() {
    stacker_setup
}

function teardown() {
    umount combined || true
    rm -rf combined || true
    umount layer0 || true
    umount layer1 || true
    rm -rf layer0 layer1 || true
    cleanup
}

@test "squashfs + derivative build only layers" {
    cat > stacker.yaml <<EOF
build:
    from:
        type: oci
        url: $CENTOS_OCI
    build_only: true
importer:
    from:
        type: built
        tag: build
    run: |
        echo hello world
EOF
    stacker build --layer-type squashfs
}

@test "squashfs yum install" {
    cat > stacker.yaml <<EOF
centos1:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        yum install -y wget
        ls /usr/bin/wget || true
EOF
    stacker build --layer-type=squashfs

    cat > stacker.yaml <<EOF
centos2:
    from:
        type: oci
        url: oci:centos1-squashfs
    run: |
        ls /usr/bin | grep wget
EOF
    stacker build --layer-type=squashfs
}

@test "squashfs import support" {
    cat > stacker.yaml <<EOF
centos1:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /1
EOF
    stacker build --layer-type=squashfs
    mv oci oci-import

    cat > stacker.yaml <<EOF
centos2:
    from:
        type: oci
        url: oci-import:centos1-squashfs
    run: |
        [ -f /1 ]
EOF
    stacker build --layer-type=squashfs
}

# the way we generate the underlying squashfs layer is different between btrfs
# and overlay, in that we glob the run: delta in with the base layer in btrfs,
# but not in overlay. so these two tests look different.
@test "squashfs layer support (btrfs)" {
    require_storage btrfs
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /1
EOF

    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)

    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0
    [ -f layer0/bin/bash ]
    [ -f layer0/1 ]
}

@test "squashfs layer support (overlay)" {
    require_storage squashfs
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /1
EOF

    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0
    [ -f layer0/bin/bash ]

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ -f layer1/1 ]
}

@test "squashfs file whiteouts (overlay)" {
    require_storage overlay
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        rm -rf /etc/selinux
        rm -f /usr/bin/ls
EOF

    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    # manually make an atomfs
    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1

    mkdir combined
    mount -t overlay -o "lowerdir=layer1:layer0" overlay combined

    # make sure directory and file whiteouts work
    [ ! -d combined/etc/selinux ]
    [ ! -f combined/usr/bin/ls ]
}

@test "squashfs + build only layers" {
    cat > stacker.yaml <<EOF
build:
    from:
        type: oci
        url: $CENTOS_OCI
    build_only: true
importer:
    from:
        type: oci
        url: $CENTOS_OCI
    import:
        - stacker://build/bin/ls
    run: |
        /stacker/ls
EOF
    stacker build --layer-type squashfs
}

@test "built type with squashfs works" {
    mkdir -p .stacker/layer-bases
    skopeo --insecure-policy copy oci:$CENTOS_OCI oci:.stacker/layer-bases/oci:centos
    umoci unpack --image .stacker/layer-bases/oci:centos dest
    tar caf .stacker/layer-bases/centos.tar -C dest/rootfs .
    rm -rf dest

	cat > stacker.yaml <<EOF
base:
  from:
    type: tar
    url: .stacker/layer-bases/centos.tar
  run: |
    echo "hello world" > /message

myroot:
  from:
    type: built
    tag: base
  run: |
    echo "foo bar" > /message
EOF
    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[1].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    cat oci/blobs/sha256/$manifest | jq -r .layers

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    cat layer1/message
    [ "$(cat layer1/message)" == "foo bar" ]
}

@test "built type with squashfs build-only base works (btrfs)" {
    require_storage btrfs
    mkdir -p .stacker/layer-bases
    skopeo --insecure-policy copy oci:$CENTOS_OCI oci:.stacker/layer-bases/oci:centos
    umoci unpack --image .stacker/layer-bases/oci:centos dest
    tar caf .stacker/layer-bases/centos.tar -C dest/rootfs .
    rm -rf dest

	cat > stacker.yaml <<EOF
base:
  from:
    type: tar
    url: .stacker/layer-bases/centos.tar
  run: |
    echo "hello world" > /message
  build_only: true

myroot:
  from:
    type: built
    tag: base
  run: |
    echo "foo bar" > /message
EOF
    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)

    cat oci/blobs/sha256/$manifest | jq -r .layers

    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0
    cat layer0/message
    [ "$(cat layer0/message)" == "foo bar" ]
}

@test "built type with squashfs build-only base works (overlay)" {
    require_storage overlay
    mkdir -p .stacker/layer-bases
    skopeo --insecure-policy copy oci:$CENTOS_OCI oci:.stacker/layer-bases/oci:centos
    umoci unpack --image .stacker/layer-bases/oci:centos dest
    tar caf .stacker/layer-bases/centos.tar -C dest/rootfs .
    rm -rf dest

	cat > stacker.yaml <<EOF
base:
  from:
    type: tar
    url: .stacker/layer-bases/centos.tar
  run: |
    echo "hello world" > /message
  build_only: true

myroot:
  from:
    type: built
    tag: base
  run: |
    echo "foo bar" > /message
EOF
    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    cat oci/blobs/sha256/$manifest | jq -r .layers

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    cat layer1/message
    [ "$(cat layer1/message)" == "foo bar" ]
}
