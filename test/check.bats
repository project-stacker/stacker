load helpers

@test "stacker check is reasonable priv overlay" {
    require_privilege priv
    stacker check
}

@test "stacker check is reasonable unpriv overlay" {
    require_privilege unpriv

    # if we don't have overlay support, stacker check should fail, otherwise it
    # should succeed
    run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --debug internal-go testsuite-check-overlay
    if [ "$status" -eq 50 ]; then
        bad_stacker check
    else
        stacker check
    fi
}
