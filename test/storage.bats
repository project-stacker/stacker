load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "btrfs -> overlay -> btrfs works" {
    require_storage btrfs # only run this once

    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $CENTOS_OCI
EOF
    stacker --storage-type=btrfs build --leave-unladen
    [ ! -f roots/test/overlay_metadata.json ]
    [ "$(cat .stacker/storage.type)" = "btrfs" ]

    stacker --storage-type=overlay build
    [ -f roots/test/overlay_metadata.json ]
    [ "$(cat .stacker/storage.type)" = "overlay" ]

    stacker --storage-type=btrfs build --leave-unladen
    [ ! -f roots/test/overlay_metadata.json ]
    [ "$(cat .stacker/storage.type)" = "btrfs" ]
}

@test "overlay -> btrfs -> overlay works" {
    require_storage btrfs # only run this once

    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        url: $CENTOS_OCI
EOF
    stacker --storage-type=overlay build
    [ -f roots/test/overlay_metadata.json ]
    [ "$(cat .stacker/storage.type)" = "overlay" ]

    stacker --storage-type=btrfs build --leave-unladen
    [ ! -f roots/test/overlay_metadata.json ]
    [ "$(cat .stacker/storage.type)" = "btrfs" ]

    stacker --storage-type=overlay build
    [ -f roots/test/overlay_metadata.json ]
    [ "$(cat .stacker/storage.type)" = "overlay" ]
}
