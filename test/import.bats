load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "different URLs with same base get re-imported" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - https://bing.com/favicon.ico
EOF
    stacker build
    # wait, people don't use bing!
    cp .stacker/imports/thing/favicon.ico bing.ico
    sed -i -e 's/bing/google/g' stacker.yaml
    stacker build
    # we should re-import google's favicon since the URL changed
    [ "$(sha bing.ico)" != "$(sha .stacker/imports/thing/favicon.ico)" ]
}

@test "importing recursively" {
    mkdir -p recursive
    touch recursive/child
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - recursive
    run: |
        [ -d /stacker/imports/recursive ]
        [ -f /stacker/imports/recursive/child ]
EOF

    stacker build
}

@test "importing stacker:// recursively" {
    mkdir -p recursive
    touch recursive/child
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - recursive
    run: |
        [ -d /stacker/imports/recursive ]
        [ -f /stacker/imports/recursive/child ]
        cp -a /stacker/imports/recursive /recursive
second:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - stacker://first/recursive
    run: |
        [ -d /stacker/imports/recursive ]
        [ -f /stacker/imports/recursive/child ]
EOF

    stacker build
}

@test "different import types" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    touch test_file2
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: test_file
          hash: $test_file_sha
        - test_file2
        - https://bing.com/favicon.ico
    run: |
        [ -f /stacker/imports/test_file ]
        [ -f /stacker/imports/test_file2 ]
        cp /stacker/imports/test_file /test_file
    build_only: true
second:
    from:
        type: built
        tag: first
    import:
        - path: stacker://first/test_file
          hash: $test_file_sha
    run: |
        [ -f /stacker/imports/test_file ]
EOF

    stacker build
}

@test "import with unmatched hash should fail" {
    touch test_file
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: test_file
          hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b856
EOF

    bad_stacker build
    echo $output | grep "is different than the actual hash"
}

@test "invalid hash should fail" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: test_file
          hash: 1234abcdef
EOF
    bad_stacker build
    echo $output | grep "is not valid"
}

@test "case insensitive hash works" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    test_file_sha_upper=${test_file_sha^^}
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: test_file
          hash: $test_file_sha
    run: |
        cp /stacker/imports/test_file /test_file
    build_only: true
second:
    from:
        type: built
        tag: first
    import:
        - path: stacker://first/test_file
          hash: $test_file_sha_upper
    run: |
        [ -f /stacker/imports/test_file ]
EOF

    stacker build
}

@test "correct sha hash is allowed for internet files" {
    wget https://google.com/favicon.ico -O google_fav
    google_sha=$(sha google_fav) || { stderr "failed sha $google_fav"; return 1; }
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: https://www.google.com/favicon.ico
          hash: $google_sha
    run: |
        [ -f /stacker/imports/favicon.ico ]
EOF

    stacker build
}


@test "invalid sha hash fails build for internet files" {
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: https://www.google.com/favicon.ico
          hash: 0d4856785d1d3c3aad3e5311e654c70c19d335f927c24ebb89dfcd52b2d988cb
EOF

    bad_stacker build
    echo $output | grep 'hash does not match'
}

@test "`require hash` flag allows build when hash is provided" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    wget https://google.com/favicon.ico -O google_fav
    google_sha=$(sha google_fav) || { stderr "failed sha $google_fav"; return 1; }
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: https://www.google.com/favicon.ico
          hash: $google_sha
        - path: test_file
          hash: $test_file_sha

    run: |
        [ -f /stacker/imports/favicon.ico ]
EOF

    stacker build --require-hash
}

@test "`require hash` flag fails build when http import hash is not provided" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    wget https://google.com/favicon.ico -O google_fav
    google_sha=$(sha google_fav) || { stderr "failed sha $google_fav"; return 1; }
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: https://www.google.com/favicon.ico
        - path: test_file
          hash: $test_file_sha
EOF

    bad_stacker build --require-hash
}

@test "`require hash` flag allows build even when local hash is not provided" {
    touch test_file
    wget https://google.com/favicon.ico -O google_fav
    google_sha=$(sha google_fav) || { stderr "failed sha $google_fav"; return 1; }
    cat > stacker.yaml <<EOF
thing:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: https://www.google.com/favicon.ico
          hash: $google_sha
        - path: test_file
EOF

    stacker build --require-hash
}

@test "invalid import " {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - "zomg"
        - - "one"
          - "two"
EOF
    bad_stacker build
}

@test "import a full directory tree with siblings" {
    cat > stacker.yaml <<EOF
busybox:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - dir
    run: |
        find /stacker
        [ -f /stacker/imports/dir/one/two/three/four/five/a ]
        [ -f /stacker/imports/dir/file ]
EOF

    mkdir -p dir/one/two/three/four/five
    touch dir/one/a
    touch dir/one/two/a
    touch dir/one/two/three/a
    touch dir/one/two/three/four/a
    touch dir/one/two/three/four/five/a
    touch dir/file

    stacker build
}

@test "different import types with perms" {
    touch test_file
    test_file_sha=$(sha test_file) || { stderr "failed sha $test_file"; return 1; }
    touch test_file2
    cat > stacker.yaml <<EOF
first:
    from:
        type: oci
        url: $BUSYBOX_OCI
    import:
        - path: test_file
          hash: $test_file_sha
        - test_file2
        - https://bing.com/favicon.ico
    run: |
        [ -f /stacker/imports/test_file ]
        [ -f /stacker/imports/test_file2 ]
        cp /stacker/imports/test_file /test_file
    build_only: true
second:
    from:
        type: built
        tag: first
    import:
        - path: stacker://first/test_file
          perms: 0777
          hash: $test_file_sha
    run: |
        [ -f /stacker/imports/test_file ]
third:
    from:
        type: scratch
    import:
        - path: stacker://first/test_file
          mode: 0777
          hash: $test_file_sha
          dest: /
fourth:
    from:
        type: scratch
    import:
        - path: test_file
          hash: $test_file_sha
          mode: 0777
          uid: 1000
          gid: 1000
          dest: /usr/bin
fifth:
  from:
    type: scratch
  import:
    - path: test_file
      dest: /files/
    - path: test_file2
      dest: /file2
sixth:
  from:
    type: docker
    url: oci:${UBUNTU_OCI}
  import:
    - stacker://fifth/files/test_file
    - stacker://fifth/file2
  run: |
    ls -l /stacker
seventh:
  from:
    type: scratch
  import:
    - path: test_file
      dest: /files/
    - path: test_file2
      dest: /file2
    - path: test_file
      dest: /files/file3
eigth:
  from:
    type: docker
    url: oci:${UBUNTU_OCI}
  import:
    - path: test_file
      dest: /dir/files/
    - path: test_file2
      dest: /dir/file2
    - path: test_file
      dest: /dir/files/file3
  run: |
    ls -alR /dir
    [ -d /dir/files ]
    [ -f /dir/files/test_file ]
    [ -f /dir/file2 ]
    [ -f /dir/files/file3 ]
EOF
    stacker build
}

@test "import with dir contents" {
  mkdir folder1
  touch folder1/file1
  mkdir folder1/subfolder2
  touch folder1/subfolder2/subfile1
  cat > stacker.yaml <<EOF
first:
  from:
    type: oci
    url: $BUSYBOX_OCI
  import:
  - path: folder1/
    dest: /folder1/
  run: |
    [ -f /folder1/file1 ]

second:
  from:
    type: built
    tag: first
  run: |
    mkdir -p /folder1
    touch /folder1/file1
    touch /folder1/file2

third:
  from:
    type: oci
    url: $BUSYBOX_OCI
  import:
    - path: stacker://second/folder1/
      dest: /folder1/
  run: |
    [ -f /folder1/file1 ]
    [ -f /folder1/file2 ]

fourth:
  from:
    type: oci
    url: $BUSYBOX_OCI
  import:
  - path: folder1/
    dest: /
  run: |
    ls -l /
    [ -f /file1 ]
    mkdir -p /folder4/subfolder5
    touch /folder4/subfolder5/subfile6

fifth:
  from:
    type: oci
    url: $BUSYBOX_OCI
  import:
  - path: folder1/subfolder2/
    dest: /folder3/
  - path: folder1/subfolder2
    dest: /folder4
  - path: stacker://fourth/folder4/subfolder5/
    dest: /folder6/
  - path: stacker://fourth/folder4/
    dest: /folder7/
  run: |
    ls -l /folder*
    [ -f /folder3/subfile1 ]
    [ ! -e /folder3/subfolder2 ]
    [ -f /folder4/subfile1 ]
    [ -f /folder6/subfile6 ]
    [ ! -e /folder6/subfolder5 ]
    [ -f /folder7/subfolder5/subfile6 ]
EOF
    stacker build
}

@test "dir path behavior" {
  mkdir -p folder1
  touch folder1/file1
  mkdir -p folder1/subfolder2
  touch folder1/subfolder2/subfile1
  cat > stacker.yaml <<EOF
src_folder_dest_non_existent_folder_case1:
  from:
    type: docker
    url: oci:${UBUNTU_OCI}
  import:
  - path: folder1
    dest: /folder2
  run: |
    [ -f /folder2/file1 ]

src_folder_dest_non_existent_folder_case2:
  from:
    type: docker
    url: oci:${UBUNTU_OCI}
  import:
  - path: folder1/
    dest: /folder2
  run: |
    [ -f /folder2/file1 ]

src_folder_dest_non_existent_folder_case3:
  from:
    type: docker
    url: oci:${UBUNTU_OCI}
  import:
  - path: folder1
    dest: /folder2/
  run: |
    ls -al /
    ls -al /folder2
    [ -f /folder2/folder1/file1 ]

src_folder_dest_non_existent_folder_case4:
  from:
    type: docker
    url: oci:${UBUNTU_OCI}
  import:
  - path: folder1/
    dest: /folder2/
  run: |
    [ -f /folder2/file1 ]
EOF
  stacker build
}
