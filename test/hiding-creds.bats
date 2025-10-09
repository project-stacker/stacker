load helpers

function setup() {
    stacker_setup

}

function teardown() {
    cleanup
}

@test "secrets loaded via imports should not be in output image" {

    cat > stacker.yaml <<"EOF"
build-with-secrets:
  from:
    type: oci
    url: ${{BUSYBOX_OCI}}
  imports:
    - creds
  run: |
    source /stacker/imports/creds
    echo "pass creds via imported files like $creds"
    echo "don't pass creds on the command line like ${{SUBCRED}}."
    # also don't do
    # echo $creds > /ohno
EOF

    filecred=$(uuidgen)
    subcred=$(uuidgen)

    cat >creds <<EOF
export creds=street,hoops,beach,$filecred
EOF

    stacker --debug build --substitute SUBCRED=$subcred --substitute BUSYBOX_OCI=$BUSYBOX_OCI

    # subcred should be in a manifest:
    run zgrep $subcred oci/blobs/sha256/*
    assert_success

    # filecred should not be in the image:
    run zgrep $filecred oci/blobs/sha256/*
    assert_failure

}


@test "secrets loaded via imports but saved to file WILL leak" {

    cat > stacker.yaml <<"EOF"
build-with-secrets:
  from:
    type: oci
    url: ${{BUSYBOX_OCI}}
  imports:
    - creds
  run: |
    source /stacker/imports/creds
    echo "pass creds via imported files like $creds"
    # but do not do this:
    echo $creds > /ohno
EOF

    filecred=$(uuidgen)

    cat >creds <<EOF
export creds=street,hoops,beach,$filecred
EOF

    stacker --debug build --substitute BUSYBOX_OCI=$BUSYBOX_OCI

    # filecred will sadly be in a manifest:
    run zgrep $filecred oci/blobs/sha256/*
    assert_success

}
