load helpers

function setup() {
    stacker_setup
}

function teardown() {
	rm -rf dir || true
    cleanup
}

@test "importing broken symlink is ok" {
    cat > stacker.yaml <<EOF
broken_link:
    from:
        type: oci
        url: $BUSYBOX_OCI
    imports:
        - dir
    run: cp -a /stacker/imports/dir/testln /testln
EOF
    mkdir -p dir
    ln -s broken dir/testln
	stacker build
    umoci unpack --image oci:broken_link dest
    [ "$status" -eq 0 ]

	[ -L dest/rootfs/testln ]
}
