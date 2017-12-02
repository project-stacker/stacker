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
    umount roots >& /dev/null || true
    rm -rf roots .stacker >& /dev/null || true
    if [ "$RESULT" != "success" ]; then
        RESULT=failure
    fi
    echo done with testing: $RESULT
}
trap cleanup EXIT HUP INT TERM

set -x

stacker build --leave-unladen -f ./basic.yaml
[ -d roots/centos ]

# did we really download the image?
[ -f .stacker/layer-bases/aHR0cDovL2ZpbGVzLnR5Y2hvLndzL2NlbnRvcy50YXIueHo= ]

# did we do a copy correctly?
[ "$(sha .stacker/imports/centos/$(echo -n ./basic.yaml | base64))" == "$(sha ./basic.yaml)" ]

RESULT=success
