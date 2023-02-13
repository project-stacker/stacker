function run_git {
    sudo -u $SUDO_USER git "$@"
}

ROOT_DIR=$(run_git rev-parse --show-toplevel)
if [ "$(id -u)" != "0" ]; then
    echo "you should be root to run this suite"
    exit 1
fi

function skip_if_no_unpriv_overlay {
    run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --debug internal-go testsuite-check-overlay
    echo $output
    [ "$status" -eq 50 ] && skip "need newer kernel for unpriv overlay"
    [ "$status" -eq 0 ]
}

function run_stacker {
    echo "Debug mode: $NO_DEBUG"
    if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
        if [[ -n "$NO_DEBUG" && "$NO_DEBUG" = 1 ]]; then
            run "${ROOT_DIR}/stacker" "$@"
        else
            run "${ROOT_DIR}/stacker" --debug "$@"
        fi
    else
        skip_if_no_unpriv_overlay
        if [[ -n "$NO_DEBUG" && "$NO_DEBUG" = 1 ]]; then
            run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" "$@"
        else
            run sudo -u $SUDO_USER "${ROOT_DIR}/stacker" --debug "$@"
        fi
    fi
}

function image_copy {
    run_stacker internal-go copy "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

STACKER_DOCKER_BASE=${STACKER_DOCKER_BASE:-docker://}
STACKER_BUILD_CENTOS_IMAGE=${STACKER_BUILD_CENTOS_IMAGE:-${STACKER_DOCKER_BASE}centos:latest}
STACKER_BUILD_UBUNTU_IMAGE=${STACKER_BUILD_UBUNTU_IMAGE:-${STACKER_DOCKER_BASE}ubuntu:latest}
GOCOVERDIR=${GOCOVERDIR:-.}
(
    flock 9
    [ -f "$ROOT_DIR/test/centos/index.json" ] || (image_copy "${STACKER_BUILD_CENTOS_IMAGE}" "oci:$ROOT_DIR/test/centos:latest" && chmod -R 777 "$ROOT_DIR/test/centos")
    [ -f "$ROOT_DIR/test/ubuntu/index.json" ] || (image_copy "${STACKER_BUILD_UBUNTU_IMAGE}" "oci:$ROOT_DIR/test/ubuntu:latest" && chmod -R 777 "$ROOT_DIR/test/ubuntu")
) 9<$ROOT_DIR/test/main.py
export CENTOS_OCI="$ROOT_DIR/test/centos:latest"
export UBUNTU_OCI="$ROOT_DIR/test/ubuntu:latest"
export GOCOVERDIR="."

function sha() {
    echo $(sha256sum $1 | cut -f1 -d" ")
}

function stacker_setup() {
    export TEST_TMPDIR=$(tmpd $BATS_TEST_NAME)
    cd $TEST_TMPDIR

    if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
        return
    fi

    "${ROOT_DIR}/stacker" unpriv-setup
    chown -R $SUDO_USER:$SUDO_USER .
}

function cleanup() {
    cd "$ROOT_DIR/test"
    umount_under "$TEST_TMPDIR"
    rm -rf "$TEST_TMPDIR" || true
}

function run_as {
    if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
        "$@"
    else
        sudo -u "$SUDO_USER" "$@"
    fi
}

function stacker {
    run_stacker "$@"
    echo "$output"
    [ "$status" -eq 0 ]
}

function bad_stacker {
    run_stacker "$@"
    echo "$output"
    [ "$status" -ne 0 ]
}

function require_privilege {
    [ "$PRIVILEGE_LEVEL" = "$1" ] || skip "test not valid for privilege level $PRIVILEGE_LEVEL"
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
