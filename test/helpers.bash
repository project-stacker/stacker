ROOT_DIR=$(git rev-parse --show-toplevel)
if [ "$(id -u)" != "0" ]; then
    echo "you should be root to run this suite"
    exit 1
fi

# now that we have a package named "oci", we can't be in the top level dir, so
# let's ensure everything is cd'd into the test/ dir. since we run stacker via
# the abspath below, this works fine.
cd "$ROOT_DIR/test"

function sha() {
    echo $(sha256sum $1 | cut -f1 -d" ")
}

function cleanup() {
    rm -rf stacker.yaml >& /dev/null || true
    umount roots >& /dev/null || true
    rm -rf roots oci dest >& /dev/null || true
    rm link >& /dev/null || true
    if [ -z "$STACKER_KEEP" ]; then
        rm -rf .stacker >& /dev/null || true
    else
        rm -rf .stacker/btrfs.loop .stacker/build.cache .stacker/imports >& /dev/null || true
    fi
}

function stacker {
    run "${ROOT_DIR}/stacker" --debug "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function bad_stacker {
    run "${ROOT_DIR}/stacker" --debug "$@"
    echo "$output"
    [ "$status" -ne 0 ]
}
