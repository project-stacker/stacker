
load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "base layer missing fails and prints" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: built
        tag: notatag
EOF
    bad_stacker build
    [[ "${output}" =~ ^(.*)$ ]]
    echo "${output}" | grep "couldn't find dependencies for test: base layer notatag"
}

@test "imports missing fails and prints" {
    cat > stacker.yaml <<EOF
test:
    from:
        type: oci
        tag: $BUSYBOX_OCI
    import:
        - stacker://foo/bar
        - stacker://baz/foo
EOF
    bad_stacker build
    echo "${output}" | grep "couldn't find dependencies for test: stacker://foo/bar, stacker://baz/foo"
}

@test "stacker:// style nesting w/ type built works" {
    cat > stacker.yaml <<EOF
minbase1:
  from:
    type: oci
    url: $UBUNTU_OCI
  run: |
    echo pkgtool install cpio pigz

installer-initrd:
    build_only: true
    from:
        type: built
        tag: minbase1
    run: |
        #!/bin/bash -ex
        mkdir /output
        touch /output/installer-initrd-base.cpio

installer-initrd-modules:
    from:
        type: built
        tag: installer-build-env
    run: |
        #!/bin/bash -ex
        mkdir -p /output
        touch /output/installer-initrd-modules.cpio

installer-build-env:
    from:
        type: built
        tag: minbase1
    build_only: true

installer-iso-build:
    from:
        type: built
        tag: minbase1
    build_only: true
    import:
        - stacker://installer-initrd/output/installer-initrd-base.cpio
        - stacker://installer-initrd-modules/output/installer-initrd-modules.cpio
    run: |
        #!/bin/bash -ex
        # populate the iso
        mkdir /output
        ( cd /stacker/imports && tar -cf /output/installer-iso.tar *.cpio )

atomix-installer-iso:
    from:
        type: tar
        url: stacker://installer-iso-build/output/installer-iso.tar
EOF
    stacker build
}
