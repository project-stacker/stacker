load helpers

function setup() {
    stacker_setup
}

function teardown() {
    if [ -f "$TEST_TMPDIR/containerd.pid" ]; then
        pid=$(cat "$TEST_TMPDIR/containerd.pid")
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
    fi
    zot_teardown
    cleanup
}

function host_arch() {
    case "$(uname -m)" in
        x86_64) echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *)
            go env GOARCH
            ;;
    esac
}

function write_containerd_config() {
    local config_file="$1"
    local hosts_config_path="$2"
    local arch
    arch=$(host_arch)

    cat > "$config_file" <<EOF
version = 3
root = '$TEST_TMPDIR/containerd-root'
state = '$TEST_TMPDIR/containerd-state'

[grpc]
  address = '$TEST_TMPDIR/containerd.sock'

[plugins.'io.containerd.service.v1.diff-service']
  default = ["erofs", "walking"]

[plugins."io.containerd.differ.v1.erofs"]
  mkfs_options = ["--sort=none"]

[[plugins."io.containerd.transfer.v1.local".unpack_config]]
  differ = "erofs"
  platform = "linux/$arch"
  snapshotter = "erofs"
    layer_types = [
        "application/vnd.stacker.image.layer.erofs",
        "application/vnd.stacker.image.layer.erofs+lz4hc",
        "application/vnd.stacker.image.layer.erofs+lz4",
        "application/vnd.stacker.image.layer.erofs+zstd"
    ]

[plugins.'io.containerd.snapshotter.v1.erofs']
  root_path = '$TEST_TMPDIR/containerd-erofs'
EOF

        if [ -n "$hosts_config_path" ]; then
                cat >> "$config_file" <<EOF

[plugins.'io.containerd.cri.v1.images'.registry]
    config_path = '$hosts_config_path'
EOF
        fi
}

function write_registry_mirror_hosts() {
        local mirror_registry="$1"
        local hosts_dir="$TEST_TMPDIR/certs.d/$mirror_registry"

        mkdir -p "$hosts_dir"
        cat > "$hosts_dir/hosts.toml" <<EOF
# For this test, make Zot the registry endpoint for docker.io so that
# both resolve and pull happen against Zot.
server = "http://${ZOT_HOST}:${ZOT_PORT}"

[host."http://${ZOT_HOST}:${ZOT_PORT}"]
  capabilities = ["pull", "resolve"]

# Optional fallback if you want it (keeps behavior stable across containerd versions):
[host."https://$mirror_registry"]
  capabilities = ["pull", "resolve"]
EOF
}

function start_containerd() {
        start_containerd_with_registry_config ""
}

function start_containerd_with_registry_config() {
        local hosts_config_path="$1"
    local containerd_bin="$ROOT_DIR/hack/tools/bin/containerd"
    local config_file="$TEST_TMPDIR/containerd.toml"
    local ctr_bin="$ROOT_DIR/hack/tools/bin/ctr"
    local n=0

        write_containerd_config "$config_file" "$hosts_config_path"

    "$containerd_bin" -c "$config_file" > "$TEST_TMPDIR/containerd.log" 2>&1 &
    echo $! > "$TEST_TMPDIR/containerd.pid"

    while [ "$n" -lt 30 ]; do
        if "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" plugins ls >/dev/null 2>&1; then
            return 0
        fi

        n=$((n+1))
        sleep 1
    done

    echo "containerd failed to start" >&3
    cat "$TEST_TMPDIR/containerd.log" >&3
    return 1
}

function ensure_erofs_ready() {
    run modinfo erofs
    [ "$status" -eq 0 ] || skip "missing erofs kernel module"

    run modprobe erofs
    [ "$status" -eq 0 ] || skip "unable to load erofs kernel module"

    run grep -Eq '^nodev[[:space:]]+erofs$|[[:space:]]erofs$' /proc/filesystems
    [ "$status" -eq 0 ] || skip "erofs filesystem is not available"
}

@test "stacker erofs image unpacks with containerd erofs snapshotter" {
    require_privilege priv

    local containerd_bin="$ROOT_DIR/hack/tools/bin/containerd"
    local ctr_bin="$ROOT_DIR/hack/tools/bin/ctr"
    local manifest_digest
    local image_ref

    [ -x "$containerd_bin" ] || skip "containerd test binary missing"
    [ -x "$ctr_bin" ] || skip "ctr test binary missing"

    ensure_erofs_ready

    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        echo hello > /hello
EOF

    stacker build --layer-type=erofs --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    manifest_digest=$(jq -r '.manifests[0].digest' oci/index.json | cut -d: -f2)
    mt="$(jq -r '.layers[0].mediaType' "oci/blobs/sha256/$manifest_digest")"
    case "$mt" in
        application/vnd.stacker.image.layer.erofs|\
        application/vnd.stacker.image.layer.erofs+lz4hc|\
        application/vnd.stacker.image.layer.erofs+lz4|\
        application/vnd.stacker.image.layer.erofs+zstd)
            ;;
        *)
            echo "unexpected EROFS layer mediaType: $mt" >&3
            return 1
            ;;
    esac

    run start_containerd
    if [ "$status" -ne 0 ]; then
        if grep -qE 'EROFS unsupported, please `modprobe erofs`|EROFS unsupported, please .*modprobe erofs' "$TEST_TMPDIR/containerd.log"; then
            skip "erofs kernel support is unavailable"
        fi
        echo "containerd failed to start for unexpected reason" >&3
        cat "$TEST_TMPDIR/containerd.log" >&3
        return 1
    fi

    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" plugins ls
    [ "$status" -eq 0 ]
    echo "$output" | grep -E "io\.containerd\.snapshotter\.v1\s+erofs\s+.*\s+ok"
    echo "$output" | grep -E "io\.containerd\.differ\.v1\s+erofs\s+.*\s+ok"

    tar -C oci -cf "$TEST_TMPDIR/stacker-erofs.oci.tar" .

    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" images import "$TEST_TMPDIR/stacker-erofs.oci.tar"
    [ "$status" -eq 0 ]

    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" images ls -q
    [ "$status" -eq 0 ]
    image_ref=$(echo "$output" | grep test-erofs | head -n1)
    [ -n "$image_ref" ]

    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" images unpack --snapshotter erofs "$image_ref"
    [ "$status" -eq 0 ]

    run find "$TEST_TMPDIR/containerd-erofs" -type f -name layer.erofs
    [ "$status" -eq 0 ]
    [ -n "$output" ]
}

@test "stacker erofs image published to zot runs through containerd mirror" {
    require_privilege priv

    local containerd_bin="$ROOT_DIR/hack/tools/bin/containerd"
    local ctr_bin="$ROOT_DIR/hack/tools/bin/ctr"
    local mirror_registry="docker.io"
    local mirror_repo="stacker-erofs-mirror-${BATS_TEST_NUMBER}"
    local mirror_ref="$mirror_registry/library/$mirror_repo:latest"

    [ -x "$containerd_bin" ] || skip "containerd test binary missing"
    [ -x "$ctr_bin" ] || skip "ctr test binary missing"
    [ -n "${ZOT_HOST}${ZOT_PORT}" ] || skip "zot env not configured"

    ensure_erofs_ready

    zot_setup

    # Ensure Zot is actually up before proceeding
    for i in $(seq 1 30); do
        if curl -fsS "http://${ZOT_HOST}:${ZOT_PORT}/v2/" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    curl -fsS "http://${ZOT_HOST}:${ZOT_PORT}/v2/" >/dev/null || {
        echo "zot health check failed" >&3
        return 1
    }

    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        echo hello-from-zot-mirror > /hello
EOF

    stacker build --layer-type=erofs --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --image library/$mirror_repo --tag latest

    write_registry_mirror_hosts "$mirror_registry"

    run start_containerd_with_registry_config "$TEST_TMPDIR/certs.d"
    if [ "$status" -ne 0 ]; then
        if grep -qE 'EROFS unsupported, please `modprobe erofs`|EROFS unsupported, please .*modprobe erofs' "$TEST_TMPDIR/containerd.log"; then
            skip "erofs kernel support is unavailable"
        fi
        echo "containerd failed to start for unexpected reason" >&3
        cat "$TEST_TMPDIR/containerd.log" >&3
        return 1
    fi

    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" plugins ls
    [ "$status" -eq 0 ]
    echo "$output" | grep -E "io\.containerd\.snapshotter\.v1\s+erofs\s+.*\s+ok"
    echo "$output" | grep -E "io\.containerd\.differ\.v1\s+erofs\s+.*\s+ok"

    # This image only exists in Zot; successful pull verifies mirror resolution.
    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" images pull "$mirror_ref"
    [ "$status" -eq 0 ]

    run "$ctr_bin" --address "$TEST_TMPDIR/containerd.sock" run --rm --snapshotter erofs "$mirror_ref" erofs-mirror-test sh -ec "cat /hello"
    [ "$status" -eq 0 ]
    echo "$output" | grep -q "hello-from-zot-mirror"

    run find "$TEST_TMPDIR/containerd-erofs" -type f -name layer.erofs
    [ "$status" -eq 0 ]
    [ -n "$output" ]
}
