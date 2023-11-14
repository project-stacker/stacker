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
        url: $BUSYBOX_OCI
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

@test "squashfs mutate /usr/bin" {
    cat > stacker.yaml <<EOF
busybox1:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        touch /usr/bin/foo
EOF
    stacker build --layer-type=squashfs

    cat > stacker.yaml <<EOF
busybox2:
    from:
        type: oci
        url: oci:busybox1-squashfs
    run: |
        ls /usr/bin | grep foo
EOF
    stacker build --layer-type=squashfs
}

@test "squashfs import support" {
    cat > stacker.yaml <<EOF
busybox1:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        touch /1
EOF
    stacker build --layer-type=squashfs
    mv oci oci-import

    cat > stacker.yaml <<EOF
busybox2:
    from:
        type: oci
        url: oci-import:busybox1-squashfs
    run: |
        [ -f /1 ]
EOF
    stacker build --layer-type=squashfs
}

@test "squashfs layer support (overlay)" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    run: |
        touch /1
EOF

    stacker build --layer-type=squashfs

    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer0=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest | cut -f2 -d:)
    layer1=$(cat oci/blobs/sha256/$manifest | jq -r .layers[1].digest | cut -f2 -d:)

    mkdir layer0
    mount -t squashfs oci/blobs/sha256/$layer0 layer0
    [ -f layer0/bin/sh ]

    mkdir layer1
    mount -t squashfs oci/blobs/sha256/$layer1 layer1
    [ -f layer1/1 ]
}

@test "squashfs file whiteouts (overlay)" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
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
        url: $BUSYBOX_OCI
    build_only: true
importer:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - stacker://build/bin/ls
    run: |
        /stacker/imports/ls
EOF
    stacker build --layer-type squashfs
}

@test "built type with squashfs works" {
    mkdir -p .stacker/layer-bases
    chmod 777 .stacker/layer-bases
    image_copy oci:$BUSYBOX_OCI oci:.stacker/layer-bases/oci:busybox
    umoci unpack --image .stacker/layer-bases/oci:busybox dest
    tar caf .stacker/layer-bases/busybox.tar -C dest/rootfs .
    rm -rf dest

	cat > stacker.yaml <<EOF
base:
  from:
    type: tar
    url: .stacker/layer-bases/busybox.tar
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

@test "built type with squashfs build-only base works (overlay)" {
    mkdir -p .stacker/layer-bases
    chmod 777 .stacker/layer-bases
    image_copy oci:$BUSYBOX_OCI oci:.stacker/layer-bases/oci:busybox
    umoci unpack --image .stacker/layer-bases/oci:busybox dest
    tar caf .stacker/layer-bases/busybox.tar -C dest/rootfs .
    rm -rf dest

	cat > stacker.yaml <<EOF
base:
  from:
    type: tar
    url: .stacker/layer-bases/busybox.tar
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

@test "build squashfs then tar" {
  echo "x" > x
  cat > stacker.yaml <<"EOF"
install-base:
  build_only: true
  from:
    type: docker
    url: "docker://zothub.io/machine/bootkit/rootfs:v0.0.17.231018-squashfs"
    
install-rootfs-pkg:
  from:
    type: built
    tag: install-base
  build_only: true
  run: |
    #!/bin/sh -ex
    writefile() {
      mkdir -p "${1%/*}"
      echo "write $1" 1>&2
      cat >"$1"
    }

    writefile /etc/systemd/network/20-wire-enp0s-dhcp.network <<"END"
    [Match]
    Name=enp0s*
    [Network]
    DHCP=yes
    END

demo-zot:
  from:
    type: built
    tag: install-rootfs-pkg
  import:
    - x
  run: |
    #!/bin/sh -ex
    cp /stacker/imports/x /usr/bin/x
EOF
  stacker build --layer-type=squashfs
  stacker build --layer-type=tar
}
