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
        url: ${{CENTOS_OCI}}
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

    out=$(stacker build --substitute bind_path=${bind_path} --substitute CENTOS_OCI=$CENTOS_OCI)

    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]

    stat tree1/foo/bar
}

@test "bind as struct" {
    cat > stacker.yaml <<"EOF"
bind-test:
    from:
        type: oci
        url: ${{CENTOS_OCI}}
    binds:
        - Source: ${{bind_path}}
          Dest: /root/tree1/foo
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

    out=$(stacker build --substitute bind_path=$bind_path  --substitute CENTOS_OCI=$CENTOS_OCI)
    [[ "${out}" =~ ^(.*filesystem bind-test built successfully)$ ]]

    stat tree1/foo/bar
}
