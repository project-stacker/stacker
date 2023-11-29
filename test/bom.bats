load helpers

function setup_file() {
  if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
    zot_setup
  fi
}

function teardown_file() {
  if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
    zot_teardown
  fi
}

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "all container contents must be accounted for" {
  skip_slow_test
  cat > stacker.yaml <<"EOF"
bom-parent:
    from:
        type: oci
        url: ${{CENTOS_OCI}}
    bom:
      generate: true
      namespace: "https://test.io/artifacts"
      packages:
      - name: pkg1
        version: 1.0.0
        license: Apache-2.0
        paths: [/pkg1]
      - name: pkg2
        version: 1.0.0
        license: Apache-2.0
        paths: [/pkg2]
    run: |
      # discover installed pkgs
      /stacker/bin/stacker bom discover
      # our own custom packages
      mkdir -p /pkg1
      touch /pkg1/file
      mkdir -p /pkg2
      touch /pkg2/file
      # should cause build to fail!
      mkdir -p /orphan-without-a-package
      touch /orphan-without-a-package/file
      # cleanup
      rm -rf /var/lib/alternatives /tmp/* \
        /etc/passwd- /etc/group- /etc/shadow- /etc/gshadow- \
        /etc/sysconfig/network /etc/nsswitch.conf.bak \
        /etc/rpm/macros.image-language-conf /var/lib/rpm/.dbenv.lock \
        /var/lib/rpm/Enhancename /var/lib/rpm/Filetriggername \
        /var/lib/rpm/Recommendname /var/lib/rpm/Suggestname \
        /var/lib/rpm/Supplementname /var/lib/rpm/Transfiletriggername \
        /var/log/anaconda \
        /etc/sysconfig/anaconda /etc/sysconfig/network-scripts/ifcfg-* \
        /etc/sysconfig/sshd-permitrootlogin /root/anaconda-* /root/original-* /run/nologin \
        /var/lib/rpm/.rpm.lock /etc/.pwd.lock /etc/BUILDTIME
    annotations:
      org.opencontainers.image.authors: bom-test
      org.opencontainers.image.vendor: bom-test
      org.opencontainers.image.licenses: MIT
EOF
    run stacker build --substitute CENTOS_OCI=${CENTOS_OCI}
    [ "$status" -ne 0 ]
    # a full inventory for this image
    [ -f .stacker/artifacts/first/inventory.json ]
    # sbom for this image shouldn't be generated
    [ ! -a .stacker/artifacts/first/first.json ]
    # building a second time also fails due to missed cache
    run stacker build
    [ "$status" -ne 0 ]
    stacker clean
}

@test "bom tool should work inside run" {
  skip_slow_test
  cat > stacker.yaml <<"EOF"
bom-parent:
    from:
        type: oci
        url: ${{CENTOS_OCI}}
    bom:
      generate: true
      namespace: "https://test.io/artifacts"
      packages:
      - name: pkg1
        version: 1.0.0
        license: Apache-2.0
        paths: [/pkg1]
      - name: pkg2
        version: 1.0.0
        license: Apache-2.0
        paths: [/pkg2]
    run: |
      # discover installed pkgs
      /stacker/bin/stacker bom discover
      # our own custom packages
      mkdir -p /pkg1
      touch /pkg1/file
      mkdir -p /pkg2
      touch /pkg2/file
      # cleanup
      rm -rf /var/lib/alternatives /tmp/* \
        /etc/passwd- /etc/group- /etc/shadow- /etc/gshadow- \
        /etc/sysconfig/network /etc/nsswitch.conf.bak \
        /etc/rpm/macros.image-language-conf /var/lib/rpm/.dbenv.lock \
        /var/lib/rpm/Enhancename /var/lib/rpm/Filetriggername \
        /var/lib/rpm/Recommendname /var/lib/rpm/Suggestname \
        /var/lib/rpm/Supplementname /var/lib/rpm/Transfiletriggername \
        /var/log/anaconda \
        /etc/sysconfig/anaconda /etc/sysconfig/network-scripts/ifcfg-* \
        /etc/sysconfig/sshd-permitrootlogin /root/anaconda-* /root/original-* /run/nologin \
        /var/lib/rpm/.rpm.lock /etc/.pwd.lock /etc/BUILDTIME
    annotations:
      org.opencontainers.image.authors: "Alice P. Programmer"
      org.opencontainers.image.vendor: "ACME Widgets & Trinkets Inc."
      org.opencontainers.image.licenses: MIT

second:
  from:
    type: built
    tag: first
  bom:
    generate: true
    namespace: "https://test.io/artifacts"
    packages:
    - name: pkg3
      version: 1.0.0
      license: Apache-2.0
      paths: [/pkg3]
  run: |
    # our own custom packages
    mkdir -p /pkg3
    touch /pkg3/file
  annotations:
      org.opencontainers.image.authors: bom-test
      org.opencontainers.image.vendor: bom-test
      org.opencontainers.image.licenses: MIT
EOF
    stacker build --substitute CENTOS_OCI=${CENTOS_OCI}
    [ -f .stacker/artifacts/bom-parent/installed-packages.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/first/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/first/first.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/second/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/second/second.json ]
    if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
      zot_setup
      stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --tag latest --substitute CENTOS_OCI=${CENTOS_OCI}
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/bom-parent:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/first:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/second:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/second:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
      zot_teardown
    fi
    stacker clean
}

@test "bom for alpine-based image" {
  cat > stacker.yaml <<"EOF"
bom-alpine:
  from:
    type: oci
    url: ${{ALPINE_OCI}}
  bom:
    generate: true
    namespace: "https://test.io/artifacts"
    packages:
      - name: pkg1
        version: 1.0.0
        license: Apache-2.0
        paths: [/file1]
  annotations:
    org.opencontainers.image.authors: bom-alpine
    org.opencontainers.image.vendor: bom-alpine
    org.opencontainers.image.licenses: MIT
  run: |
    # discover installed pkgs
    /stacker/bin/stacker bom discover
    # run our cmds
    ls -al  /
    # some changes
    touch /file1
    # cleanup
    rm -f /etc/alpine-release /etc/apk/arch /etc/apk/repositories /etc/apk/world /etc/issue /etc/os-release /etc/secfixes.d/alpine /lib/apk/db/installed /lib/apk/db/lock /lib/apk/db/scripts.tar /lib/apk/db/triggers
EOF
    stacker build --substitute ALPINE_OCI=${ALPINE_OCI}
    [ -f .stacker/artifacts/bom-alpine/installed-packages.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/bom-alpine/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/bom-alpine/bom-alpine.json ]
    if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
      zot_setup
      stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --tag latest --substitute ALPINE_OCI=${ALPINE_OCI}
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/bom-alpine:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/bom-alpine:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
    fi
    stacker clean
}

@test "pull boms if published" {
  #skip_slow_test
  cat > stacker.yaml <<EOF
parent:
    from:
        type: oci
        url: $ALPINE_OCI
    bom:
      generate: true
      namespace: "https://test.io/artifacts"
      packages:
      - name: pkg1
        version: 1.0.0
        license: Apache-2.0
        paths: [/pkg1]
      - name: pkg2
        version: 1.0.0
        license: Apache-2.0
        paths: [/pkg2]
    annotations:
      org.opencontainers.image.authors: "Alice P. Programmer"
      org.opencontainers.image.vendor: "ACME Widgets & Trinkets Inc."
      org.opencontainers.image.licenses: MIT
    run: |
      # discover installed pkgs
      /stacker/bin/stacker bom discover
      # our own custom packages
      mkdir -p /pkg1
      touch /pkg1/file
      mkdir -p /pkg2
      touch /pkg2/file
      # cleanup
      rm -rf /etc/alpine-release /etc/apk/arch /etc/apk/repositories \
            /etc/apk/world /etc/issue /etc/os-release /etc/secfixes.d/alpine \
            /lib/apk/db /etc/apk/repositories /etc/apk/world /etc/issue \
            /etc/alpine-release /etc/apk/arch /etc/os-release /etc/secfixes.d/alpine
EOF
    stacker build
    [ -f .stacker/artifacts/parent/installed-packages.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/parent/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/parent/parent.json ]
    if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
      stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --tag latest
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/parent:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/parent:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
    fi

  cat > stacker.yaml <<EOF
child:
  from:
    type: docker
    url: docker://$ZOT_HOST:$ZOT_PORT/parent:latest
    insecure: true
  bom:
    generate: true
    namespace: "https://test.io/artifacts"
    packages:
    - name: pkg3
      version: 1.0.0
      license: Apache-2.0
      paths: [/pkg3]
  annotations:
      org.opencontainers.image.authors: bom-test
      org.opencontainers.image.vendor: bom-test
      org.opencontainers.image.licenses: MIT
  run: |
    # our own custom packages
    mkdir -p /pkg3
    touch /pkg3/file
    [ ! -f /etc/apk/repositories ]
EOF
    stacker clean
    stacker build
    # a full inventory for this image
    [ -f .stacker/artifacts/child/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/child/child.json ]
    if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
      stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --tag latest
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/child:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/child:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
    fi
    stacker clean
}
