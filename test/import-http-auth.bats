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
    stop_ran_server
    cleanup
    rm -rf http_root || true
}

@test "importing from http server with auth works" {
    export XDG_RUNTIME_DIR=$TEST_TMPDIR
    mkdir -p $TEST_TMPDIR/containers/
    local valid_auth invalid_auth
    valid_auth=$(encode_auth "iam:careful")
    invalid_auth=$(encode_auth "wrong:password")
    # Include syntactically valid but incorrect credentials for the bare
    # hostname and host:port, and correct credentials for the subpath.
    #
    # NOTE: it is important that the key for each auth dict does NOT end in `/`.
    #
    # Due to the way that containers/image trims path components, it will never
    # search for a subpath with the slash at the end.
    cat > $TEST_TMPDIR/containers/auth.json <<EOF
{
    "auths": {
         "localhost": {"auth": "${invalid_auth}"},
         "localhost:9999": {"auth": "${invalid_auth}"},
         "localhost:9999/path/to": {"auth": "${valid_auth}"}
    }
}
EOF
    export REGISTRY_AUTH_FILE=$TEST_TMPDIR/containers/auth.json
    ./ran_for_stackertest -p 9999 -r http_root -a "iam:careful" \
        > ran.log 2>&1 &
    echo "$!" > "$TEST_TMPDIR/ran.pid"
    wait_for_port localhost 9999 || { cat ran.log; return 1; }
    ls -l $XDG_RUNTIME_DIR/containers/
    if ! stacker --debug build -f stacker.yaml --substitute BUSYBOX_OCI=${BUSYBOX_OCI}; then
        cat ran.log
        return 1
    fi
    umoci ls --layout oci

    umoci unpack --image oci:img img
    [ "$(sha http_root/path/to/importme)" == "$(sha img/rootfs/importme)" ]
}
