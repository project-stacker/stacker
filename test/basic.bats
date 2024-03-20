load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "multiple stacker builds in a row" {
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports: import
EOF
    echo 1 > import
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo 2 > import
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo 3 > import
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo 4 > import
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}

@test "basic workings" {
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: tar
        url: .stacker/layer-bases/busybox.tar
    imports:
        - ./stacker.yaml
        - https://www.cisco.com/favicon.ico
        - ./executable
    run:
        - cp /stacker/imports/${{FAVICON}} ${{FAVICON}}
        - ls -al /stacker
        - cp /stacker/imports/executable /usr/bin/executable
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
        tag: busybox
    run:
        - rm /favicon.ico
EOF

    touch executable
    chmod +x executable
    mkdir -p .stacker/layer-bases
    chmod 777 .stacker/layer-bases
    image_copy oci:$BUSYBOX_OCI oci:.stacker/layer-bases/oci:busybox
    umoci unpack --image .stacker/layer-bases/oci:busybox dest
    tar caf .stacker/layer-bases/busybox.tar -C dest/rootfs .
    rm -rf dest

    stacker build --substitute "FAVICON=favicon.ico"
    [ "$status" -eq 0 ]

    # did we really download the image to the right place?
    [ -f .stacker/layer-bases/busybox.tar ]

    # did run actually copy the favicon to the right place?
    stacker grab busybox:/favicon.ico
    [ "$(sha .stacker/imports/busybox/favicon.ico)" == "$(sha favicon.ico)" ]

    [ ! -f roots/layer1/rootfs/favicon.ico ] || [ ! -f roots/layer1/overlay/favicon.ico ]

    rm executable
    stacker grab busybox:/usr/bin/executable
    [ "$(stat --format="%a" executable)" = "755" ]

    # did we do a copy correctly?
    [ "$(sha .stacker/imports/busybox/stacker.yaml)" == "$(sha ./stacker.yaml)" ]

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
    sed -e 's/${{FAVICON}}/favicon.ico/g' stacker.yaml > stacker_after_subs

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
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /foo
EOF
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci unpack --image oci:busybox dest
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
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /foo
EOF
    bad_stacker --roots-dir $tmpd/with:colon build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    [ "$status" -eq 1 ]
    echo $output | grep "forbidden"
}

@test "use colons in layer name should fail" {
    local tmpd=$(pwd)
    cat > stacker.yaml <<"EOF"
busybox:with:colon:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /foo
EOF
    bad_stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    [ "$status" -eq 1 ]
    echo $output | grep "forbidden"
}

@test "basic workings with substitutions from a file" {
    cat > subs.yaml << EOF
    FAVICON: favicon.ico
EOF
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: tar
        url: .stacker/layer-bases/busybox.tar
    import:
    imports:
        - ./stacker.yaml
        - https://www.cisco.com/favicon.ico
        - ./executable
    run:
        - cp /stacker/imports/${{FAVICON}} ${{FAVICON}}
        - ls -al /stacker
        - cp /stacker/imports/executable /usr/bin/executable
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
        tag: busybox
    run:
        - rm /favicon.ico
EOF

    touch executable
    chmod +x executable
    mkdir -p .stacker/layer-bases
    chmod 777 .stacker/layer-bases
    image_copy oci:$BUSYBOX_OCI oci:.stacker/layer-bases/oci:busybox
    umoci unpack --image .stacker/layer-bases/oci:busybox dest
    tar caf .stacker/layer-bases/busybox.tar -C dest/rootfs .
    rm -rf dest

    stacker build --substitute-file subs.yaml
    [ "$status" -eq 0 ]

    # did we really download the image to the right place?
    [ -f .stacker/layer-bases/busybox.tar ]

    # did run actually copy the favicon to the right place?
    stacker grab busybox:/favicon.ico
    [ "$(sha .stacker/imports/busybox/favicon.ico)" == "$(sha favicon.ico)" ]

    [ ! -f roots/layer1/rootfs/favicon.ico ] || [ ! -f roots/layer1/overlay/favicon.ico ]

    rm executable
    stacker grab busybox:/usr/bin/executable
    [ "$(stat --format="%a" executable)" = "755" ]

    # did we do a copy correctly?
    [ "$(sha .stacker/imports/busybox/stacker.yaml)" == "$(sha ./stacker.yaml)" ]

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
    sed -e 's/${{FAVICON}}/favicon.ico/g' stacker.yaml > stacker_after_subs

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

@test "commas in substitute flags ok" {
    cat > stacker.yaml <<"EOF"
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /foo
EOF
    stacker build --substitute "a=b,c" --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}

@test "Test whiteouts across layers" {
    # /aaa is created in l1, removed in l2, re-created in l3
    # /bbb is created in l2, removed in l3
    # /ccc is created in l2, removed and recreated in l3
    # /ddd is created in l1, removed and recreated in l2, and extended in l3
    cat > stacker.yaml <<"EOF"
l1:
    from:
       type: tar
       url: .stacker/layer-bases/busybox.tar
    run: |
       mkdir -p /aaa/111/ab
       mkdir -p /ddd/111/ab
l2:
    from:
       type: built
       tag: l1
    run: |
       rm -rf /aaa
       mkdir -p /bbb/111/ab
       mkdir -p /ccc/111/ab
       rm -rf /ddd
       mkdir -p /ddd/222/ab
l3:
    from:
       type: built
       tag: l2
    run: |
       rm -rf /bbb
       rm -rf /ccc
       mkdir -p /aaa/222/ab
       mkdir -p /ccc/222/ab
       mkdir -p /ddd/333/ab
l4:
    from:
      type: built
      tag: l3
    run: |
      [ ! -d /aaa/111 ]
      [ -d /aaa/222/ab ]
      [ -d /ccc/222 ]
      [ ! -d /ccc/111 ]
EOF
    mkdir -p .stacker/layer-bases
    chmod 777 .stacker/layer-bases
    image_copy oci:$BUSYBOX_OCI oci:.stacker/layer-bases/oci:busybox
    umoci unpack --image .stacker/layer-bases/oci:busybox dest
    tar caf .stacker/layer-bases/busybox.tar -C dest/rootfs .
    rm -rf dest
    # did we really download the image to the right place?
    [ -f .stacker/layer-bases/busybox.tar ]

    stacker build
    umoci unpack --image oci:l3 l3

    # aaa/111 should be deleted, aaa/222 should exist
    [ ! -d l3/rootfs/aaa/111 ]
    [ -d l3/rootfs/aaa/222/ab ]

    # bbb should be deleted entirely
    [ ! -d l3/rootfs/bbb ]

    # ccc should be like aaa - but doesn't have an intermediate layer
    [ -d l3/rootfs/ccc/222 ]
    [ ! -d l3/rootfs/ccc/111 ]

    # ddd should have both 222 and 333 but not 111
    # This is to test specifically that the opaque xattr is not copied
    # up, causing 222 to be missed.
    [ -d l3/rootfs/ddd/333 ]
    [ -d l3/rootfs/ddd/222 ]
    [ ! -d l3/rootfs/ddd/111 ]
}
