load helpers

function setup() {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: tar
        url: .stacker/layer-bases/centos.tar
    import:
        - ./stacker.yaml
        - https://www.cisco.com/favicon.ico
        - ./executable
    run:
        - cp /stacker/\$FAVICON /\$FAVICON
        - cp /stacker/executable /usr/bin/executable
    entrypoint: echo hello world
    environment:
        FOO: bar
    volumes:
        - /data/db
    labels:
        foo: bar
        bar: baz
    working_dir: /meshuggah/rocks
layer1:
    from:
        type: built
        tag: centos
    run:
        - rm /favicon.ico
EOF

    touch executable
    chmod +x executable
}

function teardown() {
    cleanup
    rm executable
}

@test "basic workings" {
    mkdir -p .stacker/layer-bases
    skopeo --insecure-policy copy docker://centos:latest oci:.stacker/layer-bases/oci:centos
    umoci unpack --image .stacker/layer-bases/oci:centos dest
    tar caf .stacker/layer-bases/centos.tar -C dest/rootfs .
    rm -rf dest

    stacker build --substitute "FAVICON=favicon.ico" --leave-unladen
    [ "$status" -eq 0 ]

    # did we really download the image to the right place?
    [ -f .stacker/layer-bases/centos.tar ]

    # did run actually copy the favicon to the right place?
    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]

    [ ! -f roots/layer1/rootfs/favicon.ico ]

    [ "$(stat --format="%a" roots/centos/rootfs/usr/bin/executable)" = "755" ]

    # did we do a copy correctly?
    [ "$(sha .stacker/imports/centos/stacker.yaml)" == "$(sha ./stacker.yaml)" ]

    # check OCI image generation
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Entrypoint | join(" ")')" = "echo hello world" ]

    stackerGit=$(cat oci/blobs/sha256/$manifest | jq -r '.annotations."com.github.anuvu.stacker.git_version"')
    [ "stacker version ${stackerGit}" = "$(stacker --version)" ]

    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Env[0]')" = "FOO=bar" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Volumes["/data/db"]')" = "{}" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Labels["foo"]')" = "bar" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Labels["bar"]')" = "baz" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.WorkingDir')" = "/meshuggah/rocks" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.author')" = "$SUDO_USER@$(hostname)" ]

    manifest2=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    [ "$manifest" = "$manifest2" ]
    layer2=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
    [ "$layer" = "$layer2" ]

    # let's check that the main tar stuff is understood by umoci
    umoci unpack --image oci:layer1 dest
    [ ! -f dest/rootfs/favicon.ico ]
    [ ! -d dest/rootfs/stacker ]

    # Now does `stacker unlade` work?
    umount roots
    rm -rf .stacker/btrfs.loop
    stacker unlade
    [ -f roots/centos/rootfs/favicon.ico ]
    [ ! -f roots/layer1/rootfs/favicon.ico ]
}
