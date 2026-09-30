load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "rootless build does not rewrite the source git index" {
    require_privilege unpriv

    local gitdir="$TEST_TMPDIR/source-git"
    mkdir "$gitdir"
    echo hello > "$gitdir/payload"
    cat > "$gitdir/stacker.yaml" <<"EOF"
repro:
    from:
        type: scratch
    imports:
        - path: payload
          dest: /payload
EOF
    give_user_ownership "$gitdir"

    # A root build user would hide the 0:0 rewrite.
    local uid owner
    uid=$(id -u "$SUDO_USER")
    owner=$(stat -c %u:%g "$gitdir/payload")
    echo "build user: $SUDO_USER uid=$uid, payload owner: $owner" >&3
    [ "$uid" -ne 0 ]
    [ "${owner%%:*}" = "$uid" ]

    run_as git -C "$gitdir" init -q
    run_as git -C "$gitdir" add payload stacker.yaml
    run_as git -C "$gitdir" -c user.name=test -c user.email=test@example.com commit -qm init

    # Index entries cache the file owner; a refresh inside the userns rewrites it as 0:0.
    local owner_before owner_after index_before
    owner_before=$(run_as git -C "$gitdir" ls-files --debug payload | awk '/uid:/ {print $2 ":" $4}')
    index_before=$(sha "$gitdir/.git/index")
    [ "$owner_before" = "$owner" ]

    stacker build -f "$gitdir/stacker.yaml"

    owner_after=$(run_as git -C "$gitdir" ls-files --debug payload | awk '/uid:/ {print $2 ":" $4}')
    echo "index owner before: $owner_before after: $owner_after" >&3
    [ "$owner_after" = "$owner" ]
    [ "$(sha "$gitdir/.git/index")" = "$index_before" ]
}
