load helpers

function setup() {
    stacker_setup
    cat > stacker.yaml <<"EOF"
img:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - path: http://localhost:9999/importme
    run: |
        cp /stacker/imports/importme /importme
EOF

    mkdir -p http_root
    echo "please" > http_root/importme

    wget --quiet https://github.com/m3ng9i/ran/releases/download/v0.1.6/ran_linux_amd64.zip
    unzip ran_linux_amd64.zip
    mv ran_linux_amd64 ran_for_stackertest
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
    cat > $TEST_TMPDIR/containers/auth.json <<EOF
{
    "auths": {
         "localhost": {"auth": "aWFtOmNhcmVmdWw="}
    }
}
EOF
    ./ran_for_stackertest -p 9999 -r http_root -a "iam:careful" &
    ls -l $XDG_RUNTIME_DIR/containers/
    stacker --debug build -f stacker.yaml --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    umoci ls --layout oci

    umoci unpack --image oci:img img
    [ "$(sha http_root/importme)" == "$(sha img/rootfs/importme)" ]
}
