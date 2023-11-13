load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "bind as string slice" {
    cat > stacker.yaml <<"EOF"
bind-test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    binds:
        - ${{bind_path}} -> /root/tree1/foo
    run: |
        touch /root/tree1/foo/bar
EOF
    mkdir -p tree1/foo

    # since we are creating directory as
    # real root and then `touch`-ing a file
    # where in user NS, need to have rw persmission
    # for others
    chmod +666 tree1/foo

    bind_path=$(realpath tree1/foo)

    out=$(stacker build --substitute bind_path=${bind_path} --substitute BUSYBOX_OCI=$BUSYBOX_OCI)

    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]

    stat tree1/foo/bar
}

@test "bind as struct" {
    cat > stacker.yaml <<"EOF"
bind-test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    binds:
        - source: ${{bind_path1}}
          dest: /root/tree1/foo
        - source: ${{bind_path2}}
    run: |
        touch /root/tree1/foo/bar
        [ -f "${{bind_path2}}/file1" ]
EOF
    mkdir -p tree1/foo tree2/bar
    touch tree2/bar/file1

    # since we are creating directory as
    # real root and then `touch`-ing a file
    # where in user NS, need to have rw persmission
    # for others
    chmod +666 tree1/foo

    bind_path1=$(realpath tree1/foo)
    bind_path2=$(realpath tree2/bar)

    out=$(stacker build \
        "--substitute=bind_path1=${bind_path1}" \
        "--substitute=bind_path2=${bind_path2}" \
        "--substitute=BUSYBOX_OCI=$BUSYBOX_OCI" ) || {
             printf "%s\n" "$out" 1>&2
             exit 1
        }
    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]

    stat tree1/foo/bar
}
