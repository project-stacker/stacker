load helpers

function setup() {
    stacker_setup
    zot_setup_auth
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: docker
        url: docker://${ZOT_HOST}:${ZOT_PORT}/busybox:latest
EOF

}

function teardown() {
    zot_teardown
    cleanup
}

@test "from: authenticated zot works" {
    if [ -z "${ZOT_HOST}${ZOT_PORT}" ]; then
        echo "skipping test $BATS_SUITE_TEST_NUMBER because it requires running zot."
        return 0
    fi

    export XDG_RUNTIME_DIR=$TEST_TMPDIR
    mkdir -p "$XDG_RUNTIME_DIR/containers"
    cat > $TEST_TMPDIR/containers/auth.json <<EOF
{
     "auths": {
          "${ZOT_HOST}:${ZOT_PORT}": {"auth": "aWFtOmNhcmVmdWw="}
     }
}
EOF
    export REGISTRY_AUTH_FILE=$TEST_TMPDIR/containers/auth.json
    cp "$BATS_SUITE_TMPDIR/ca.crt" "$TEST_TMPDIR/ca.crt"
    chmod 0644 "$TEST_TMPDIR/ca.crt"
    export SSL_CERT_FILE=$TEST_TMPDIR/ca.crt
    skopeo copy --dest-creds "iam:careful" oci:${BUSYBOX_OCI} docker://${ZOT_HOST}:${ZOT_PORT}/busybox:latest

    stacker --debug build
}

@test "publish uses credentials from netrc for authenticated zot" {
    require_privilege priv

    if [ -z "${ZOT_HOST}${ZOT_PORT}" ]; then
        echo "skipping test $BATS_SUITE_TEST_NUMBER because it requires running zot."
        return 0
    fi

    cat > stacker.yaml <<'EOF'
busybox:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
EOF
    stacker build --substitute "BUSYBOX_OCI=${BUSYBOX_OCI}"

    export NETRC="$TEST_TMPDIR/netrc"
    cat > "$NETRC" <<EOF
machine ${ZOT_HOST}:${ZOT_PORT} login iam password careful
EOF

    stacker publish --substitute "BUSYBOX_OCI=${BUSYBOX_OCI}" \
        --skip-tls --url "docker://${ZOT_HOST}:${ZOT_PORT}" --tag netrc-test

    run skopeo inspect --tls-verify=false --creds "iam:careful" \
        "docker://${ZOT_HOST}:${ZOT_PORT}/busybox:netrc-test"
    [ "$status" -eq 0 ]
}
