load helpers

function setup() {
    stacker_setup
}

function teardown() {
    stop_ran_server
    zot_teardown
    cleanup
    rm -rf http_root || true
}

function write_auth_files() {
    local explicit_auth="$1"
    local fallback_auth="$2"

    export XDG_RUNTIME_DIR="$TEST_TMPDIR"
    mkdir -p "$XDG_RUNTIME_DIR/containers"
    printf '{"auths":{%s}}\n' "$fallback_auth" \
        > "$XDG_RUNTIME_DIR/containers/auth.json"
    printf '{"auths":{%s}}\n' "$explicit_auth" \
        > "$TEST_TMPDIR/auth.json"
    export REGISTRY_AUTH_FILE="$TEST_TMPDIR/auth.json"
}

@test "REGISTRY_AUTH_FILE takes precedence for registry image pulls" {
    if [ -z "${ZOT_HOST}${ZOT_PORT}" ]; then
        echo "skipping test $BATS_SUITE_TEST_NUMBER because it requires running zot."
        return 0
    fi

    zot_setup_auth
    local valid_auth invalid_auth
    valid_auth=$(encode_auth "iam:careful")
    invalid_auth=$(encode_auth "wrong:password")
    # The explicit auth file has Zot's valid credentials; the XDG fallback has
    # invalid credentials for the same registry. The pull checks precedence.
    write_auth_files \
        "\"${ZOT_HOST}:${ZOT_PORT}\":{\"auth\":\"${valid_auth}\"}" \
        "\"${ZOT_HOST}:${ZOT_PORT}\":{\"auth\":\"${invalid_auth}\"}"
    cp "$BATS_SUITE_TMPDIR/ca.crt" "$TEST_TMPDIR/ca.crt"
    chmod 0644 "$TEST_TMPDIR/ca.crt"
    export SSL_CERT_FILE="$TEST_TMPDIR/ca.crt"

    _skopeo copy --dest-creds "iam:careful" \
        "oci:${BUSYBOX_OCI}" \
        "docker://${ZOT_HOST}:${ZOT_PORT}/busybox:latest"

    cat > stacker.yaml <<EOF
busybox:
    from:
        type: docker
        url: docker://${ZOT_HOST}:${ZOT_PORT}/busybox:latest
EOF

    stacker --debug build
}

@test "REGISTRY_AUTH_FILE takes precedence for HTTP imports" {
    local GOARCH
    GOARCH=$(go env GOARCH)
    local valid_auth invalid_auth
    valid_auth=$(encode_auth "iam:careful")
    invalid_auth=$(encode_auth "wrong:password")

    # The explicit file has valid credentials only for the import path. The
    # fallback file has invalid credentials at every matching path, so the
    # import succeeds only when REGISTRY_AUTH_FILE is used.
    write_auth_files \
        "\"localhost\":{ \"auth\":\"${invalid_auth}\" }, \"localhost:9999\":{ \"auth\":\"${invalid_auth}\" }, \"localhost:9999/path/to\":{ \"auth\":\"${valid_auth}\" }" \
        "\"localhost\":{ \"auth\":\"${invalid_auth}\" }, \"localhost:9999\":{ \"auth\":\"${invalid_auth}\" }, \"localhost:9999/path/to\":{ \"auth\":\"${invalid_auth}\" }"

    cat > stacker.yaml <<'EOF'
img:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    imports:
        - path: http://localhost:9999/path/to/importme
    run: |
        cp /stacker/imports/importme /importme
EOF

    mkdir -p http_root/path/to
    echo "please" > http_root/path/to/importme

    wget --quiet "https://github.com/m3ng9i/ran/releases/download/v0.1.6/ran_linux_${GOARCH}.zip"
    unzip "ran_linux_${GOARCH}.zip"
    mv "ran_linux_${GOARCH}" ran_for_stackertest
    chmod +x ran_for_stackertest
    ./ran_for_stackertest -p 9999 -r http_root -a "iam:careful" \
        > ran.log 2>&1 &
    echo "$!" > "$TEST_TMPDIR/ran.pid"
    wait_for_port localhost 9999 || { cat ran.log; return 1; }

    if ! stacker --debug build -f stacker.yaml \
        --substitute "BUSYBOX_OCI=${BUSYBOX_OCI}"; then
        cat ran.log
        return 1
    fi
    umoci unpack --image oci:img img
    cmp_files http_root/path/to/importme img/rootfs/importme
}
