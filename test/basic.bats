load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "multiple stacker builds in a row" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    import: import
EOF
    echo 1 > import
    stacker build
    echo 2 > import
    stacker build
    echo 3 > import
    stacker build
    echo 4 > import
    stacker build
}

@test "basic workings" {
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
        - ls -al /stacker
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
    runtime_user: 1000
layer1:
    from:
        type: built
        tag: centos
    run:
        - rm /favicon.ico
EOF

    touch executable
    chmod +x executable
    mkdir -p .stacker/layer-bases
    chmod 777 .stacker/layer-bases
    image_copy oci:$CENTOS_OCI oci:.stacker/layer-bases/oci:centos
    umoci unpack --image .stacker/layer-bases/oci:centos dest
    tar caf .stacker/layer-bases/centos.tar -C dest/rootfs .
    rm -rf dest

    stacker build --substitute "FAVICON=favicon.ico"
    [ "$status" -eq 0 ]

    # did we really download the image to the right place?
    [ -f .stacker/layer-bases/centos.tar ]

    # did run actually copy the favicon to the right place?
    stacker grab centos:/favicon.ico
    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha favicon.ico)" ]

    [ ! -f roots/layer1/rootfs/favicon.ico ] || [ ! -f roots/layer1/overlay/favicon.ico ]

    rm executable
    stacker grab centos:/usr/bin/executable
    [ "$(stat --format="%a" executable)" = "755" ]

    # did we do a copy correctly?
    [ "$(sha .stacker/imports/centos/stacker.yaml)" == "$(sha ./stacker.yaml)" ]

    # check OCI image generation
    manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    layer=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
    config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Entrypoint | join(" ")')" = "echo hello world" ]

    publishedGitVersion=$(cat oci/blobs/sha256/$manifest | jq -r '.annotations."io.stackeroci.stacker.git_version"')
    # ci does not clone tags. There it tests the fallback-to-commit path.
    myGitVersion=$(run_git describe --tags) || myGitVersion=$(run_git rev-parse HEAD)
    [ -n "$(run_git status --porcelain --untracked-files=no)" ] &&
        dirty="-dirty" || dirty=""
    [ "$publishedGitVersion" = "$myGitVersion$dirty" ]

    # need to trim the extra newline from jq
    cat oci/blobs/sha256/$manifest | jq -r '.annotations."io.stackeroci.stacker.stacker_yaml"' | sed '$ d' > stacker_yaml_annotation

    # now we need to do --substitute FAVICON=favicon.ico
    sed -e 's/$FAVICON/favicon.ico/g' stacker.yaml > stacker_after_subs

    diff -U5 stacker_yaml_annotation stacker_after_subs

    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Env[0]')" = "FOO=bar" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.User')" = "1000" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Volumes["/data/db"]')" = "{}" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Labels["foo"]')" = "bar" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.Labels["bar"]')" = "baz" ]
    [ "$(cat oci/blobs/sha256/$config | jq -r '.config.WorkingDir')" = "/meshuggah/rocks" ]

    # TODO: this kind of sucks and is backwards, but now when running as a
    # privileged container, stacker's code will render $SUDO_USER as the user.
    # However, when running as an unprivileged user, the re-exec will cause
    # stacker to think that it is running as root, and render the author as
    # root. We could/should fix this, but AFAIK nobody pays attention to this
    # anyway...
    if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
        [ "$(cat oci/blobs/sha256/$config | jq -r '.author')" = "$SUDO_USER@$(hostname)" ]
    else
        [ "$(cat oci/blobs/sha256/$config | jq -r '.author')" = "root@$(hostname)" ]
    fi
    cat oci/blobs/sha256/$config | jq -r '.author'

    manifest2=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
    [ "$manifest" = "$manifest2" ]
    layer2=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
    [ "$layer" = "$layer2" ]

    # let's check that the main tar stuff is understood by umoci
    umoci unpack --image oci:layer1 dest
    [ ! -f dest/rootfs/favicon.ico ]
    [ ! -d dest/rootfs/stacker ]
}

@test "stacker.yaml without imports can run" {
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /foo
EOF
    stacker build
    umoci unpack --image oci:centos dest
    [ -f dest/rootfs/foo ]
}

@test "stacker without arguments prints help" {
    # need to manually call the binary, since our wrapper functions pass
    # --debug. also, urfave/cli is the one that actually handles this case, and
    # they exit(0) for now :(
    "${ROOT_DIR}/stacker" | grep "COMMANDS"
}

@test "use colons in roots-dir path name should fail" {
    local tmpd=$(pwd)
    cat > stacker.yaml <<EOF
centos:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /foo
EOF
    bad_stacker --roots-dir $tmpd/with:colon build
    [ "$status" -eq 1 ]
    echo $output | grep "forbidden"
} 

@test "use colons in layer name should fail" {
    local tmpd=$(pwd)
    cat > stacker.yaml <<EOF
centos:with:colon:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /foo
EOF
    bad_stacker build
    [ "$status" -eq 1 ]
    echo $output | grep "forbidden"
} 
