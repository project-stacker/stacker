#!/bin/bash -e

if [ -z "$GOPATH" ]; then
    echo "no GOPATH, try sudo -E ./main.sh"
    exit 1
fi

if [ "$(id -u)" != "0" ]; then
    echo "you should be root to run this suite"
    exit 1
fi

PATH=$PATH:$GOPATH/bin

function sha() {
    echo $(sha256sum $1 | cut -f1 -d" ")
}

function cleanup() {
    set +x
    if [ "$RESULT" != "success" ]; then
        if [ -n "$STACKER_INSPECT" ]; then
            echo "waiting for inspection; press enter to continue cleanup"
            read -r foo
        fi
        RESULT=failure
    fi
    umount roots >& /dev/null || true
    rm -rf roots >& /dev/null || true
    if [ -z "$STACKER_KEEP" ]; then
        rm -rf .stacker >& /dev/null || true
    else
        rm -rf .stacker/logs .stacker/btrfs.loop
    fi
    echo done with testing: $RESULT
}
trap cleanup EXIT HUP INT TERM

# clean up old logs if they exist
if [ -n "$STACKER_KEEP" ]; then
    rm -rf .stacker/logs .stacker/btrfs.loop
fi

set -x

stacker build --leave-unladen -f ./basic.yaml
[ -d roots/centos ]

# did we really download the image?
[ -f .stacker/layer-bases/centos.tar.xz ]

# did we do a copy correctly?
[ "$(sha .stacker/imports/centos/basic.yaml)" == "$(sha ./basic.yaml)" ]

RESULT=success
