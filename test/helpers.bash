ROOT_DIR=$(git rev-parse --show-toplevel)
if [ "$(id -u)" != "0" ]; then
    echo "you should be root to run this suite"
    exit 1
fi

export CENTOS_OCI="$ROOT_DIR/test/centos:latest"
export UBUNTU_OCI="$ROOT_DIR/test/ubuntu:latest"

function skip_if_no_unpriv_overlay {
    [ "$STORAGE_TYPE" == "overlay" ] || return 0
    run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --debug internal-go testsuite-check-overlay
    echo $output
    [ "$status" -eq 50 ] && skip "need newer kernel for unpriv overlay"
    [ "$status" -eq 0 ]
}

function sha() {
    echo $(sha256sum $1 | cut -f1 -d" ")
}

function stacker_setup() {
    export TEST_TMPDIR=$(tmpd $BATS_TEST_NAME)
    cd $TEST_TMPDIR
}

function cleanup() {
    cd "$ROOT_DIR/test"
    umount_under "$TEST_TMPDIR"
    rm -rf "$TEST_TMPDIR" || true
}

function stacker {
    run "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE --debug "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function bad_stacker {
    run "${ROOT_DIR}/stacker" --debug "$@"
    echo "$output"
    [ "$status" -ne 0 ]
}

function unpriv_stacker {
    skip_if_no_unpriv_overlay
    run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE --debug "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function unpriv_setup {
    stacker unpriv-setup
    chown -R $SUDO_USER:$SUDO_USER .
}

function strace_stacker {
    run strace -f -s 4096 "${ROOT_DIR}/stacker" --debug "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function image_copy {
    run "${ROOT_DIR}/stacker" internal-go copy "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function stacker_chroot {
    chroot_run=$(mktemp -p ${TEST_TMPDIR} chroot_runfile.XXXXXXX)
    chroot_stderr=$(mktemp -p ${TEST_TMPDIR} chroot_stderr.XXXXXXX)
    cat "${ROOT_DIR}/test/helpers.bash" >> "${chroot_run}"
    echo "$@" >> "${chroot_run}"
    cat "${chroot_run}" | "${ROOT_DIR}/stacker" --storage-type=$STORAGE_TYPE --debug chroot 2>"${chroot_stderr}"
    [ "$?" -eq 0 ] || (cat "${chroot_stderr}" && false)
}

function require_storage {
    [ "$STORAGE_TYPE" = "$1" ] || skip "test not valid for storage type $STORAGE_TYPE"
}

function tmpd() {
    mktemp -d "${PWD}/stackertest${1:+-$1}.XXXXXX"
}

function stderr() {
    echo "$@" 1>&2
}

function umount_under() {
    # umount_under(dir)
    # unmount dir and anything under it.
    # note IFS gets set to '\n' by bats.
    local dir="" mounts="" mp="" oifs="$IFS"
    [ -d "$1" ] || return 0
    # make sure its a full path.
    dir=$(realpath $1)
    # reverse the entries to unwind.
    mounts=$(awk '
        $2 ~ matchdir || $2 == dir { found=$2 "|" found; };
        END { printf("%s\n", found); }' \
            "dir=$dir" matchdir="^${dir}/" /proc/mounts)
    IFS="|"; set -- ${mounts}; IFS="$oifs"
    [ $# -gt 0 ] || return 0
    for mp in "$@"; do
        umount "$mp" || {
            stderr "failed umount $mp."
            return 1
        }
    done
    return 0
}

function cmp_files() {
    local f1="$1" f2="$2" f1sha="" f2sha=""
    [ -f "$f1" ] || { stderr "$f1: not a file"; return 1; }
    [ -f "$f2" ] || { stderr "$f2: not a file"; return 1; }
    f1sha=$(sha "$f1") || { stderr "failed sha $f1"; return 1; }
    f2sha=$(sha "$f2") || { stderr "failed sha $f2"; return 1; }
    if [ "$f1sha" != "$f2sha" ]; then
        stderr "$f1 and $f2 differed"
        diff -u "$f1" "$f2" 1>&2 || :
        return 1
    fi
    return 0
}
