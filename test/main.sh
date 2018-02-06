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
    umount roots >& /dev/null || true
    rm -rf roots oci dest >& /dev/null || true
    if [ -z "$STACKER_KEEP" ]; then
        rm -rf .stacker >& /dev/null || true
    else
        rm -rf .stacker/logs .stacker/btrfs.loop .stacker/build.cache
    fi
    echo done with testing: $RESULT
}

function on_exit() {
    set +x
    if [ "$RESULT" != "success" ]; then
        if [ -n "$STACKER_INSPECT" ]; then
            echo "waiting for inspection; press enter to continue cleanup"
            read -r foo
        fi
        RESULT=failure
    fi
    cleanup
}
trap on_exit EXIT HUP INT TERM

# clean up old logs if they exist
if [ -n "$STACKER_KEEP" ]; then
    rm -rf .stacker/logs .stacker/btrfs.loop
fi

set -x

function check_image() {
    # did run actually copy the favicon to the right place?
    [ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]

    [ ! -f roots/layer1/rootfs/favicon.ico ]

    [ "$(stat --format="%a" roots/centos/rootfs/usr/bin/executable)" = "755" ]
}

# Do a build.
stacker build --substitute "FAVICON=favicon.ico" --leave-unladen -f ./basic.yaml

check_image

# did we really download the image?
[ -f .stacker/layer-bases/centos.tar.xz ]

# did we do a copy correctly?
[ "$(sha .stacker/imports/centos/basic.yaml)" == "$(sha ./basic.yaml)" ]

# check OCI image generation
manifest=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
layer=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
config=$(cat oci/blobs/sha256/$manifest | jq -r .config.digest | cut -f2 -d:)
[ "$(cat oci/blobs/sha256/$config | jq -r '.config.Entrypoint | join(" ")')" = "echo hello world" ]

[ "$(cat oci/blobs/sha256/$config | jq -r '.config.Env[0]')" = "FOO=bar" ]
[ "$(cat oci/blobs/sha256/$config | jq -r '.config.Volumes["/data/db"]')" = "{}" ]
[ "$(cat oci/blobs/sha256/$config | jq -r '.config.Labels["foo"]')" = "bar" ]
[ "$(cat oci/blobs/sha256/$config | jq -r '.config.Labels["bar"]')" = "baz" ]
[ "$(cat oci/blobs/sha256/$config | jq -r '.config.WorkingDir')" = "/meshuggah/rocks" ]

manifest2=$(cat oci/index.json | jq -r .manifests[0].digest | cut -f2 -d:)
[ "$manifest" = "$manifest2" ]
layer2=$(cat oci/blobs/sha256/$manifest | jq -r .layers[0].digest)
[ "$layer" = "$layer2" ]

# let's check that the main tar stuff is understood by umoci
umoci unpack --image oci:layer1 dest
[ ! -f dest/rootfs/favicon.ico ]

# Now does `stacker unlade` work?
umount roots
rm -rf .stacker/btrfs.loop
stacker unlade
[ -f roots/centos/rootfs/favicon.ico ]
[ ! -f roots/layer1/rootfs/favicon.ico ]

cleanup

# now, let's do something really crazy: import a docker image and build our own
# layer on top of it.
stacker build --leave-unladen -f ./import-docker.yaml
[ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]
umoci unpack --image oci:layer1 dest
[ ! -f dest/rootfs/favicon.ico ]

cleanup

# Ok, now let's try unprivileged stacker.
truncate -s 100G .stacker/btrfs.loop
mkfs.btrfs .stacker/btrfs.loop
mkdir -p roots
mount -o loop .stacker/btrfs.loop roots
chown -R $SUDO_USER:$SUDO_USER roots
chown -R $SUDO_USER:$SUDO_USER .stacker
sudo -u $SUDO_USER $GOPATH/bin/stacker build -f ./import-docker.yaml
umoci unpack --image oci:layer1 dest

[ "$(sha .stacker/imports/centos/favicon.ico)" == "$(sha roots/centos/rootfs/favicon.ico)" ]
[ ! -f dest/rootfs/favicon.ico ]

cleanup

# Do build only layers work?
stacker build -f buildonly.yaml
umoci unpack --image oci:layer1 dest
[ "$(sha dest/rootfs/favicon.ico)" == "$(sha dest/rootfs/favicon2.ico)" ]
[ "$(umoci ls --layout ./oci)" == "$(printf "centos-latest\nlayer1")" ]

cleanup

# Do scratch layers work?
stacker build -f scratch.yaml
umoci unpack --image oci:empty dest
[ "$(ls dest/rootfs)" == "" ]

RESULT=success
