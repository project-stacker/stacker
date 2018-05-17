if [ "$(id -u)" != "0" ]; then
    echo "you should be root to run this suite"
    exit 1
fi

if [ -z "$GOPATH" ]; then
    echo "need to set $GOPATH"
fi

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
        rm -rf .stacker/btrfs.loop .stacker/build.cache >& /dev/null || true
    fi
}

function stacker {
    run $GOPATH/bin/stacker --debug "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function bad_stacker {
    run $GOPATH/bin/stacker --debug "$@"
    echo "$output"
    [ "$status" -ne 0 ]
}
