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
    require_privilege priv

    export XDG_RUNTIME_DIR=$TEST_TMPDIR
    mkdir -p $TEST_TMPDIR/containers/
    cat > $TEST_TMPDIR/containers/auth.json <<EOF
{
     "auths": {
          "${ZOT_HOST}:${ZOT_PORT}": {"auth": "aWFtOmNhcmVmdWw="}
     }
}
EOF
    export SSL_CERT_FILE=$BATS_SUITE_TMPDIR/ca.crt
    skopeo copy --dest-creds "iam:careful" oci:${BUSYBOX_OCI} docker://${ZOT_HOST}:${ZOT_PORT}/busybox:latest

    stacker --debug build
}
