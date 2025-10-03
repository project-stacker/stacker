load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'
load 'test_helper/bats-file/load'

function run_git {
    sudo -u $SUDO_USER git "$@"
}

ROOT_DIR=$(run_git rev-parse --show-toplevel)
if [ "$(id -u)" != "0" ]; then
    echo "you should be root to run this suite"
    exit 1
fi

function give_user_ownership() {
   if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
      return
   fi
   if [ -z "$SUDO_UID" ]; then
      echo "PRIVILEGE_LEVEL=$PRIVILEGE_LEVEL but empty SUDO_USER"
      exit 1
   fi
   chown -R "$SUDO_USER:$SUDO_USER" "$@"
}

function skip_if_no_unpriv_overlay {
    local wdir=""
    # use a workdir to ensure no side effects to the caller
    wdir=$(mktemp -d "$PWD/.skipunpriv.XXXXXX")
    give_user_ownership "$wdir"
    run sudo -u $SUDO_USER \
        "${ROOT_DIR}/stacker" "--work-dir=$wdir" --debug \
            internal-go testsuite-check-overlay
    rm -Rf "$wdir"
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
STACKER_BUILD_ALPINE_IMAGE=${STACKER_BUILD_ALPINE_IMAGE:-${STACKER_DOCKER_BASE}alpine:edge}
STACKER_BUILD_BUSYBOX_IMAGE=${STACKER_BUILD_BUSYBOX_IMAGE:-${STACKER_DOCKER_BASE}busybox:latest}
STACKER_BUILD_CENTOS_IMAGE=${STACKER_BUILD_CENTOS_IMAGE:-${STACKER_DOCKER_BASE}centos:latest}
STACKER_BUILD_UBUNTU_IMAGE=${STACKER_BUILD_UBUNTU_IMAGE:-${STACKER_DOCKER_BASE}ubuntu:latest}
(
    flock 9
    [ -f "$ROOT_DIR/test/alpine/index.json" ] || (image_copy "${STACKER_BUILD_ALPINE_IMAGE}" "oci:$ROOT_DIR/test/alpine:edge" && chmod -R 777 "$ROOT_DIR/test/alpine")
    [ -f "$ROOT_DIR/test/busybox/index.json" ] || (image_copy "${STACKER_BUILD_BUSYBOX_IMAGE}" "oci:$ROOT_DIR/test/busybox:latest" && chmod -R 777 "$ROOT_DIR/test/busybox")
    [ -f "$ROOT_DIR/test/centos/index.json" ] || (image_copy "${STACKER_BUILD_CENTOS_IMAGE}" "oci:$ROOT_DIR/test/centos:latest" && chmod -R 777 "$ROOT_DIR/test/centos")
    [ -f "$ROOT_DIR/test/ubuntu/index.json" ] || (image_copy "${STACKER_BUILD_UBUNTU_IMAGE}" "oci:$ROOT_DIR/test/ubuntu:latest" && chmod -R 777 "$ROOT_DIR/test/ubuntu")
) 9<$ROOT_DIR/test/main.py
export ALPINE_OCI="$ROOT_DIR/test/alpine:edge"
export BUSYBOX_OCI="$ROOT_DIR/test/busybox:latest"
export CENTOS_OCI="$ROOT_DIR/test/centos:latest"
export UBUNTU_OCI="$ROOT_DIR/test/ubuntu:latest"
export PATH="$ROOT_DIR/hack/tools/bin:$PATH"

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

function skip_slow_test {
    case "${SLOW_TEST:-false}" in
        true) return;;
        false) skip "${BATS_TEST_NAME} is slow. Set SLOW_TEST=true to run.";;
        *) stderr "SLOW_TEST variable must be 'true' or 'false'" \
            "found '${SLOW_TEST}'"
           return 1;;
    esac
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

function write_plain_zot_config {
  cat > $TEST_TMPDIR/zot-config.json << EOF
{
  "distSpecVersion": "1.1.0-dev",
  "storage": {
    "rootDirectory": "$TEST_TMPDIR/zot",
    "gc": true,
    "dedupe": true
  },
  "http": {
    "address": "$ZOT_HOST",
    "port": "$ZOT_PORT"
  },
  "log": {
    "level": "debug",
    "output": "$TEST_TMPDIR/zot.log"
  }
}
EOF

}

function write_auth_zot_config {

  htpasswd -Bbn iam careful >> $TEST_TMPDIR/htpasswd

  cat > $TEST_TMPDIR/zot-config.json << EOF
{
  "distSpecVersion": "1.1.0-dev",
  "storage": {
    "rootDirectory": "$TEST_TMPDIR/zot",
    "gc": true,
    "dedupe": true
  },
  "http": {
    "tls": {
      "cert": "$BATS_SUITE_TMPDIR/server.cert",
      "key": "$BATS_SUITE_TMPDIR/server.key"
    },
    "address": "$ZOT_HOST",
    "port": "$ZOT_PORT",
    "auth": {
      "htpasswd": {
        "path": "$TEST_TMPDIR/htpasswd"
      }
    },
    "accessControl": {
      "repositories": {
        "**": {
          "policies": [{
              "users": [ "iam" ],
              "actions": [ "read", "create", "update" ]
          }]
        }
      }
    }
  },
  "log": {
    "level": "debug",
    "output": "$TEST_TMPDIR/zot.log",
    "audit": "$TEST_TMPDIR/zot-audit.log"
  }
}
EOF

}

function zot_setup {
    write_plain_zot_config
    start_zot
}

function zot_setup_auth {
    write_auth_zot_config
    start_zot USE_TLS
}

function start_zot {
  ZOT_USE_TLS=$1
  echo "# starting zot at $ZOT_HOST:$ZOT_PORT" >&3
  # start as a background task
  zot verify $TEST_TMPDIR/zot-config.json
  zot serve $TEST_TMPDIR/zot-config.json &
  pid=$!

  echo "zot is running at pid $pid"
  cat $TEST_TMPDIR/zot.log
  # wait until service is up
  count=5
  up=0

  while [[ $count -gt 0 ]]; do
    if [ ! -d /proc/$pid ]; then
      echo "zot failed to start or died"
      exit 1
    fi
    up=1
    if [[ -n $ZOT_USE_TLS ]]; then
        echo "testing zot at https://$ZOT_HOST:$ZOT_PORT"
        curl -v --cacert $BATS_SUITE_TMPDIR/ca.crt  -u "iam:careful" -f https://$ZOT_HOST:$ZOT_PORT/v2/ || up=0
    else
        echo "testing zot at http://$ZOT_HOST:$ZOT_PORT"
        curl -v -f http://$ZOT_HOST:$ZOT_PORT/v2/ || up=0
    fi

    if [ $up -eq 1 ]; then break; fi
    sleep 1
    count=$((count - 1))
  done
  if [ $up -eq 0 ]; then
    echo "Timed out waiting for zot"
    exit 1
  fi

  echo "# zot is up" >&3
  # setup a OCI client
  if [[ -n $ZOT_USE_TLS ]]; then
      regctl registry set $ZOT_HOST:$ZOT_PORT
  else
      regctl registry set --tls=disabled $ZOT_HOST:$ZOT_PORT
  fi

}

function zot_teardown {
  echo "# stopping zot" >&3
  killall zot
  killall -KILL zot || true
  rm -f $TEST_TMPDIR/zot-config.json
  rm -rf $TEST_TMPDIR/zot
}

function _skopeo() {
    [ "$1" = "--version" ] && {
        skopeo "$@"
        return
    }
    local uid=""
    uid=$(id -u)
    if [ ! -e /run/containers ]; then
        if [ "$uid" = "0" ]; then
            mkdir --mode=755 /run/containers || chmod /run/containers 755
        fi
    fi
    [ -n "$TEST_TMPDIR" ]
    local home="${TEST_TMPDIR}/home"
    [ -d "$home" ] || mkdir -p "$home"
    HOME="$home" skopeo "$@"
}

function dir_has_only() {
    local d="$1" oifs="$IFS" unexpected="" f=""
    shift
    _RET_MISSING=""
    _RET_EXTRA=""
    unexpected=$(
        shopt -s nullglob;
        IFS="/"; allexp="/$*/"; IFS="$oifs"
        # allexp gets /item/item2/ for all items in args
        x=""
        cd "$d" || {
            echo "dir_has_only could not 'cd $d' from $PWD" 1>&2;
            exit 1;
        }
        for found in * .*; do
            [ "$found" = "." ] || [ "$found" = ".." ] && continue
            [ "${allexp#*/$found/}" != "$allexp" ] && continue
            x="$x $found"
        done
        echo "$x"
    ) || return 1
    _RET_EXTRA="${unexpected# }"
    for f in "$@"; do
        [ -e "$d/$f" -o -L "$d/$f" ] && continue
        _RET_MISSING="${_RET_MISSING} $f"
    done
    _RET_MISSING="${_RET_MISSING# }"
    [ -z "$_RET_MISSING" -a -z "${_RET_EXTRA}" ]
    return
}

function dir_is_empty() {
    dir_has_only "$1"
}

# log a failure with ERROR:
# allows more descriptive error:
#   [ -f file ] || test_error "missing 'file'"
# compared to just
#   [ -f file ]
function test_error() {
    local m=""
    echo "ERROR: $1"
    shift
    for m in "$@"; do
        echo "  $m"
    done
    return 1
}

function test_copy_buffer_size() {
   local buffer_size=$1
   local file_type=$2
   
   # create a temporary dir
   local tmpdir=$(mktemp -d "$BATS_TEST_TMPDIR"/copy${1:+-$1}.XXXXXX)
   cd "$tmpdir"
   if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
     return
   fi

   "${ROOT_DIR}/stacker" unpriv-setup
   chown -R $SUDO_USER:$SUDO_USER .

   mkdir folder1
   truncate -s $buffer_size folder1/file1
   if [ $file_type = "tar" ]
   then
     tar cvf test.$file_type folder1
   elif [ $file_type = "tar.gz" ]
   then
     tar cvzf test.$file_type folder1
   else
    echo "unknown file type: $file_type"
    exit 1
   fi
   cat > stacker.yaml <<EOF
tar:
  from:
    type: tar
    url: test.$file_type
EOF
  stacker build
  cat oci/index.json | jq .
  m1=$(cat oci/index.json | jq .manifests[0].digest | sed  's/sha256://' | tr -d \")
  cat oci/blobs/sha256/"$m1" | jq .
  l1=$(cat oci/blobs/sha256/"$m1" | jq .layers[0].digest | sed  's/sha256://' | tr -d \")
  _skopeo --version
  [[ "$(_skopeo --version)" =~ "skopeo version ${SKOPEO_VERSION}" ]] || {
    echo "skopeo --version should be ${SKOPEO_VERSION}"
    exit 1
  }
  _skopeo copy --format=oci oci:oci:tar containers-storage:test:tar
  _skopeo copy --format=oci containers-storage:test:tar oci:oci:test
  cat oci/index.json | jq .
  m2=$(cat oci/index.json | jq .manifests[1].digest | sed  's/sha256://' | tr -d \")
  cat oci/blobs/sha256/"$m2" | jq .
  l2=$(cat oci/blobs/sha256/"$m2" | jq .layers[0].digest | sed  's/sha256://' | tr -d \")
  echo "$l1"
  echo "$l2"
  [ "$l1" = "$l2" ]
  stacker clean
  rm -rf folder1
  cd "$ROOT_DIR"
  rm -rf "$tmpdir"
}
