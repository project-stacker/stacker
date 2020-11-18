load helpers

function setup() {
    stacker_setup
}

function teardown() {
    rm -rf a b || true
    cleanup
}

@test "container-setup: two containers works" {
    require_storage btrfs
    cat > stacker.yaml <<EOF
a:
    from:
        type: oci
        url: $CENTOS_OCI
    binds:
        - ./a -> /a
b:
    from:
        type: oci
        url: $CENTOS_OCI
    binds:
        - ./b -> /b
EOF

    stacker container-setup
    mkdir -p roots a b
    mount -o loop .stacker/btrfs.loop "roots"

    # stacker marks its subvols as read-only, so we need to mark them as
    # writable if we want lxc to be able to mkdir any bind mount directories it
    # needs for the binds: specified in the stacker.yaml
    btrfs property set -ts "roots/a" ro false
    btrfs property set -ts "roots/b" ro false

    # start the containers a few times to make sure they work.
    lxc-start -F --name=a -P "$ROOT_DIR/test" -f "roots/a/lxc.conf" -- sh -c "[ -f /a ]"
    lxc-start -F --name=b -P "$ROOT_DIR/test" -f "roots/b/lxc.conf" -- sh -c "[ -f /b ]"
    lxc-start -F --name=a -P "$ROOT_DIR/test" -f "roots/a/lxc.conf" -- sh -c "[ -f /a ]"
    lxc-start -F --name=b -P "$ROOT_DIR/test" -f "roots/b/lxc.conf" -- sh -c "[ -f /b ]"
}

@test "container-setup generates a config" {
    require_storage btrfs
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $CENTOS_OCI
    build_env:
        FOO: bar
EOF
    stacker container-setup
    mkdir -p roots
    mount -o loop .stacker/btrfs.loop roots
    grep "FOO=bar" "roots/test/lxc.conf"
}
