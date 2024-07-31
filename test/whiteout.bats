load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "test not adding extraneous whiteouts" {
    cat > stacker.yaml <<EOF
image:
  from:
    type: docker
    url: docker://public.ecr.aws/ubuntu/ubuntu:latest
  run: |
    apt-get update
    apt-get -y install libsensors-config
EOF

  stacker build
  echo "checking"
  for f in $(ls oci/blobs/sha256/); do
    file oci/blobs/sha256/$f | grep "gzip" || {
      echo "skipping $f"
        continue
    }
    bsdtar -tvf oci/blobs/sha256/$f
    run "bsdtar -tvf oci/blobs/sha256/$f | grep '.wh.sensors.d'"
    if [ "$status" -eq 0 ]; then
      echo "should not have a sensors.d whiteout!";
      exit 1;
    fi
  done
}

@test "dont emit whiteout for new dir creates" {
  cat > stacker.yaml <<EOF
  # a1.tar has /a1/file
bb:
  from:
    type: docker
    url: docker://busybox
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

  stacker build
}
