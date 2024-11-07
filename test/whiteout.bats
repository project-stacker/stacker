load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "test not adding extraneous whiteouts" {
    cat > stacker.yaml <<"EOF"
image:
  from:
    type: oci
    url: ${{UBUNTU_OCI}}
  run: |
    apt-get update
    apt-get -y install libsensors-config
EOF

  stacker build --substitute UBUNTU_OCI=${UBUNTU_OCI}
  echo "checking"
  for f in oci/blobs/sha256/*; do
    file oci/blobs/sha256/$(basename $f) | grep "gzip" || {
      echo "skipping $f"
        continue
    }
    bsdtar -tvf $f
    # we expect the grep to fail, if it returns success we fail the test since
    # it means we have .wh files in the tar which we should NOT.
    if run bsdtar -tvf $f | grep '.wh.sensors.d'; then
      echo "should not have a sensors.d whiteout!";
      exit 1;
    fi
  done
}

@test "dont emit whiteout for new dir creates" {
  cat > stacker.yaml <<"EOF"
  # a1.tar has /a1/file
bb:
  from:
    type: oci
    url: ${{BUSYBOX_OCI}}
  run: |
    mkdir /a1
    touch /a1/file

nodir:
  from:
    type: built
    tag: bb
  run: |
    rm -rf /a1

emptydir:
  from:
    type: built
    tag: bb
  run: |
    rm -rf /a1
    mkdir /a1

fulldir:
  from:
    type: built
    tag: bb
  run: |
    rm -rf /a1
    mkdir /a1
    touch /a1/newfile
EOF

  stacker build --substitute BUSYBOX_OCI=${BUSYBOX_OCI}
}
