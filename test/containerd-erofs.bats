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
  layer_types = ["vnd.erofs.layer.overlayfs.v1.erofs"]

[plugins.'io.containerd.snapshotter.v1.erofs']
  root_path = '$TEST_TMPDIR/containerd-erofs'
EOF
}

function start_containerd() {
    local containerd_bin="$ROOT_DIR/hack/tools/bin/containerd"
    local config_file="$TEST_TMPDIR/containerd.toml"
    local ctr_bin="$ROOT_DIR/hack/tools/bin/ctr"
    local n=0

    write_containerd_config "$config_file"

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

@test "stacker erofs image unpacks with containerd erofs snapshotter" {
    require_privilege priv

    local containerd_bin="$ROOT_DIR/hack/tools/bin/containerd"
    local ctr_bin="$ROOT_DIR/hack/tools/bin/ctr"
    local manifest_digest
    local image_ref

    [ -x "$containerd_bin" ] || skip "containerd test binary missing"
    [ -x "$ctr_bin" ] || skip "ctr test binary missing"

    run modinfo erofs
    [ "$status" -eq 0 ] || skip "missing erofs kernel module"

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
    # OCI format prefixes custom layer types with application/vnd.oci.image.layer.
    mt="$(jq -r '.layers[0].mediaType' "oci/blobs/sha256/$manifest_digest")"
    [ "$mt" = "application/vnd.oci.image.layer.vnd.erofs.layer.overlayfs.v1.erofs" ] || \
    [ "$mt" = "application/vnd.erofs.layer.overlayfs.v1.erofs" ]

    run start_containerd
    [ "$status" -eq 0 ]

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
