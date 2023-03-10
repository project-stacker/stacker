load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "workdir args" {
    cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $CENTOS_OCI
child:
    from:
        type: built
        tag: parent
    run: |
        echo hello world
EOF
  # check defaults
  tmpdir=$(mktemp -d)
  chmod -R a+rwx $tmpdir
  stacker --work-dir $tmpdir build
  [ -d $tmpdir ]
  [ -d $tmpdir/.stacker ]
  [ -d $tmpdir/roots ]
  [ -d $tmpdir/oci ]
  rm -rf $tmpdir

  # check overrides
  tmpdir=$(mktemp -d)
  chmod -R a+rwx $tmpdir
  stackerdir=$(mktemp -d)
  chmod -R a+rwx $stackerdir
  stacker --work-dir $tmpdir --stacker-dir $stackerdir build
  [ -d $tmpdir ]
  [ ! -d $tmpdir/.stacker ]
  [ -d $tmpdir/roots ]
  [ -d $tmpdir/oci ]
  [ -d $stackerdir ]
  rm -rf $tmpdir
  rm -rf $stackerdir
}
