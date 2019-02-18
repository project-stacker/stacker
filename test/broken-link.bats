load helpers

function setup() {
    cat > stacker.yaml <<EOF
broken_link:
    from:
        type: docker
        url: docker://centos:latest
    import:
        - dir
    run: cp -a /stacker/dir/testln /testln
EOF
    mkdir -p dir
    ln -s broken dir/testln
}

function teardown() {
	rm -rf dir || true
    cleanup
}

@test "importing broken symlink is ok" {
	stacker build
    umoci unpack --image oci:broken_link dest
    [ "$status" -eq 0 ]

	[ -L dest/rootfs/testln ]
}
