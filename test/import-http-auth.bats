load helpers

function setup() {
    GOARCH=$(go env GOARCH)
    stacker_setup
    cat > stacker.yaml <<"EOF"
img:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - path: http://localhost:9999/path/to/importme
    run: |
        cp /stacker/imports/importme /importme
EOF

    mkdir -p http_root/path/to/
    echo "please" > http_root/path/to/importme

    wget --quiet https://github.com/m3ng9i/ran/releases/download/v0.1.6/ran_linux_${GOARCH}.zip
    unzip ran_linux_${GOARCH}.zip
    mv ran_linux_${GOARCH} ran_for_stackertest
    chmod +x ran_for_stackertest


}

function teardown() {
    cleanup
    rm -rf http_root || true
    killall ran_for_stackertest || true
}

@test "importing from http server with auth works" {
    require_privilege priv

    export XDG_RUNTIME_DIR=$TEST_TMPDIR
    mkdir -p $TEST_TMPDIR/containers/
    # include valid but wrong creds for the bare hostname and host:port, and
    # correct creds for a subpath
    #
    # NOTE: it is important that the key for each auth dict does NOT end in `/`.
    #
    # Due to the way that containers/image trims path components, it will never
    # search for a subpath with the slash at the end.
    cat > $TEST_TMPDIR/containers/auth.json <<EOF
{
    "auths": {
         "localhost": {"auth": "d3Jvbmc6cGFzc3dvcmQ="},
         "localhost:9999": {"auth": "d3Jvbmc6cGFzc3dvcmQ="},
         "localhost:9999/path/to": {"auth": "aWFtOmNhcmVmdWw="}
    }
}
EOF
    ./ran_for_stackertest -p 9999 -r http_root -a "iam:careful" &
    ls -l $XDG_RUNTIME_DIR/containers/
    stacker --debug build -f stacker.yaml --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci ls --layout oci

    umoci unpack --image oci:img img
    [ "$(sha http_root/path/to/importme)" == "$(sha img/rootfs/importme)" ]
}
