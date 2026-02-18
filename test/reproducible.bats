load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

# Helper to extract config blob path from the OCI layout
function get_config_path() {
    local oci_dir="${1:-oci}"
    local manifest config
    manifest=$(cat "$oci_dir/index.json" | jq -r .manifests[0].digest | cut -f2 -d:)
    config=$(cat "$oci_dir/blobs/sha256/$manifest" | jq -r .config.digest | cut -f2 -d:)
    echo "$oci_dir/blobs/sha256/$config"
}

# Helper to extract manifest blob path from the OCI layout
function get_manifest_path() {
    local oci_dir="${1:-oci}"
    local manifest
    manifest=$(cat "$oci_dir/index.json" | jq -r .manifests[0].digest | cut -f2 -d:)
    echo "$oci_dir/blobs/sha256/$manifest"
}

@test "SOURCE_DATE_EPOCH sets config created timestamp" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/reproducible.conf
        mkdir -p /var/data
        echo "build artifact" > /var/data/output.txt
EOF
    # 2023-11-14T22:13:20Z
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    config_path=$(get_config_path)
    created=$(cat "$config_path" | jq -r '.created')

    echo "created: $created"
    # The timestamp should match the SOURCE_DATE_EPOCH value
    [ "$created" = "2023-11-14T22:13:20Z" ]
}

@test "SOURCE_DATE_EPOCH sets author to stacker" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/reproducible.conf
        mkdir -p /var/data
        echo "build artifact" > /var/data/output.txt
EOF
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    config_path=$(get_config_path)
    author=$(cat "$config_path" | jq -r '.author')

    echo "author: $author"
    # When SOURCE_DATE_EPOCH is set, author should be stabilized to "stacker"
    [ "$author" = "stacker" ]
}

@test "SOURCE_DATE_EPOCH produces reproducible builds" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/reproducible.conf
        mkdir -p /var/data
        echo "build artifact" > /var/data/output.txt
EOF
    # First build
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    manifest1=$(cat oci/index.json | jq -r .manifests[0].digest)
    config1=$(cat oci/blobs/sha256/$(echo "$manifest1" | cut -f2 -d:) | jq -r .config.digest)
    layers1=$(cat oci/blobs/sha256/$(echo "$manifest1" | cut -f2 -d:) | jq -r '[.layers[].digest] | sort | join(",")')

    echo "Build 1 - manifest: $manifest1, config: $config1"

    # Clean and rebuild with the same epoch
    stacker clean
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    manifest2=$(cat oci/index.json | jq -r .manifests[0].digest)
    config2=$(cat oci/blobs/sha256/$(echo "$manifest2" | cut -f2 -d:) | jq -r .config.digest)
    layers2=$(cat oci/blobs/sha256/$(echo "$manifest2" | cut -f2 -d:) | jq -r '[.layers[].digest] | sort | join(",")')

    echo "Build 2 - manifest: $manifest2, config: $config2"

    # Config digests should match (same timestamps, same author)
    [ "$config1" = "$config2" ]

    # Layer digests should match (same tar timestamps)
    [ "$layers1" = "$layers2" ]

    # Manifest digests should match (same config + layers)
    [ "$manifest1" = "$manifest2" ]
}

@test "SOURCE_DATE_EPOCH history timestamps use epoch" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/reproducible.conf
        mkdir -p /var/data
        echo "build artifact" > /var/data/output.txt
EOF
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    config_path=$(get_config_path)

    # Check that the last history entry uses the epoch timestamp
    last_history_created=$(cat "$config_path" | jq -r '.history[-1].created')
    echo "last history created: $last_history_created"
    [ "$last_history_created" = "2023-11-14T22:13:20Z" ]

    # Check the history created_by includes "stacker build"
    last_history_created_by=$(cat "$config_path" | jq -r '.history[-1].created_by')
    echo "last history created_by: $last_history_created_by"
    [[ "$last_history_created_by" == *"stacker build"* ]]
}

@test "without SOURCE_DATE_EPOCH author is user@hostname" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/test.conf
        echo "content" > /tmp/output.txt
EOF
    # Ensure SOURCE_DATE_EPOCH is NOT set
    unset SOURCE_DATE_EPOCH
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    config_path=$(get_config_path)
    author=$(cat "$config_path" | jq -r '.author')

    echo "author: $author"
    # Without SOURCE_DATE_EPOCH, author should be user@hostname (not "stacker")
    [ "$author" != "stacker" ]
    # Author should contain @ (user@hostname format)
    [[ "$author" == *"@"* ]]
}

@test "invalid SOURCE_DATE_EPOCH causes error" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /tmp/test.txt
EOF
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH="not-a-number"
    bad_stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
    echo "$output"
    [[ "$output" == *"invalid SOURCE_DATE_EPOCH"* ]]
}

@test "SOURCE_DATE_EPOCH with multi-layer produces reproducible builds" {
    cat > stacker.yaml <<"EOF"
base:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        mkdir -p /opt/base
        echo "base content" > /opt/base/base.txt
        touch /etc/base.conf
app:
    from:
        type: built
        tag: base
    run: |
        mkdir -p /opt/app
        echo "app content" > /opt/app/app.txt
        touch /etc/app.conf
EOF
    # First build
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    # Capture digests for the "app" layer (second manifest)
    manifest_count=$(cat oci/index.json | jq -r '.manifests | length')
    echo "manifest count: $manifest_count"

    # Find the app manifest
    app_manifest_digest=""
    for i in $(seq 0 $((manifest_count - 1))); do
        ref=$(cat oci/index.json | jq -r ".manifests[$i].annotations[\"org.opencontainers.image.ref.name\"]")
        if [ "$ref" = "app" ]; then
            app_manifest_digest=$(cat oci/index.json | jq -r ".manifests[$i].digest" | cut -f2 -d:)
            break
        fi
    done
    [ -n "$app_manifest_digest" ]

    app_config1=$(cat oci/blobs/sha256/$app_manifest_digest | jq -r .config.digest)
    app_layers1=$(cat oci/blobs/sha256/$app_manifest_digest | jq -r '[.layers[].digest] | sort | join(",")')

    echo "Build 1 - app config: $app_config1, layers: $app_layers1"

    # Clean and rebuild
    stacker clean
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    # Re-extract app manifest
    app_manifest_digest2=""
    for i in $(seq 0 $((manifest_count - 1))); do
        ref=$(cat oci/index.json | jq -r ".manifests[$i].annotations[\"org.opencontainers.image.ref.name\"]")
        if [ "$ref" = "app" ]; then
            app_manifest_digest2=$(cat oci/index.json | jq -r ".manifests[$i].digest" | cut -f2 -d:)
            break
        fi
    done
    [ -n "$app_manifest_digest2" ]

    app_config2=$(cat oci/blobs/sha256/$app_manifest_digest2 | jq -r .config.digest)
    app_layers2=$(cat oci/blobs/sha256/$app_manifest_digest2 | jq -r '[.layers[].digest] | sort | join(",")')

    echo "Build 2 - app config: $app_config2, layers: $app_layers2"

    # Both builds should produce identical results
    [ "$app_config1" = "$app_config2" ]
    [ "$app_layers1" = "$app_layers2" ]
    [ "$app_manifest_digest" = "$app_manifest_digest2" ]
}

@test "SOURCE_DATE_EPOCH different values produce different timestamps" {
    cat > stacker.yaml <<"EOF"
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        touch /etc/reproducible.conf
        mkdir -p /var/data
        echo "build artifact" > /var/data/output.txt
EOF

    # Build with one epoch
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    config_path=$(get_config_path)
    created1=$(cat "$config_path" | jq -r '.created')
    echo "created1: $created1"

    # Clean and build with a different epoch
    stacker clean

    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1600000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    config_path=$(get_config_path)
    created2=$(cat "$config_path" | jq -r '.created')
    echo "created2: $created2"

    # Timestamps should differ
    [ "$created1" != "$created2" ]
    [ "$created1" = "2023-11-14T22:13:20Z" ]
    [ "$created2" = "2020-09-13T12:26:40Z" ]
}

@test "SOURCE_DATE_EPOCH is available inside run section" {
    cat > stacker.yaml <<'EOF'
test:
    from:
        type: oci
        url: ${{BUSYBOX_OCI}}
    run: |
        echo "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}" > /epoch.txt
        # verify the value is what we set
        [ "$SOURCE_DATE_EPOCH" = "1700000000" ]
        touch /etc/reproducible.conf
EOF
    # shellcheck disable=SC2031
    export SOURCE_DATE_EPOCH=1700000000
    stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}

    # If we got here the build succeeded, meaning the [ test ] inside
    # the run section passed and SOURCE_DATE_EPOCH was available.

    # Also verify the file was written with the correct value
    umoci unpack --image oci:test dest
    echo "epoch.txt contents: $(cat dest/rootfs/epoch.txt)"
    [ "$(cat dest/rootfs/epoch.txt)" = "SOURCE_DATE_EPOCH=1700000000" ]
}
