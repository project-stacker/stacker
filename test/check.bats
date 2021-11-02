load helpers

@test "stacker check is reasonable priv btrfs" {
    require_privilege priv
    require_storage btrfs
    stacker check
}

@test "stacker check is reasonable unpriv btrfs" {
    require_privilege unpriv
    require_storage btrfs

    # assuming we're not on btrfs currently, this will fail...
    bad_stacker check

    # but after setup, it will succeed
    stacker_setup
    stacker check
}

@test "stacker check is reasonable priv overlay" {
    require_privilege priv
    require_storage overlay
    stacker check
}

@test "stacker check is reasonable unpriv overlay" {
    require_privilege unpriv
    require_storage overlay

    # if we don't have overlay support, stacker check should fail, otherwise it
    # should succeed
    run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --debug internal-go testsuite-check-overlay
    if [ "$status" -eq 50 ]; then
        bad_stacker check
    else
        stacker check
    fi
}
