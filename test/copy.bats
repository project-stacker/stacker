load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "built type layers are restored correctly" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $CENTOS_OCI
    run: |
        touch /root/parent
        cat /proc/self/mountinfo
child:
    from:
        type: built
        tag: parent
    copy:
      - source: stacker://parent/root/parent
        dest: /root/parent
        perms: 0744
        user: user
        group: group
    run: |
        cat /proc/self/mountinfo
        [ -f /root/parent ]
        touch /root/child
EOF
    stacker build

    umoci --log=debug unpack --image oci:parent dest/parent
    [ "$status" -eq 0 ]
    [ -f dest/parent/rootfs/root/parent ]

    umoci --log info unpack --image oci:child dest/child  # say my name say my name
    [ "$status" -eq 0 ]
    [ -f dest/child/rootfs/root/child ]
}
