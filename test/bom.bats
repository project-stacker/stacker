load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "all container contents must be accounted for" {
  skip_slow_test
  cat > stacker.yaml <<EOF
bom-parent:
    from:
        type: oci
        url: $CENTOS_OCI
    bom:
      generate: true
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
      /stacker/tools/static-stacker bom discover
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
    run stacker build
    [ "$status" -ne 0 ]
    # a full inventory for this image
    [ -f .stacker/artifacts/bom-parent/inventory.json ]
    # sbom for this image shouldn't be generated
    [ ! -a .stacker/artifacts/bom-parent/bom-parent.json ]
    # building a second time also fails due to missed cache
    run stacker build
    [ "$status" -ne 0 ]
    stacker clean
}

@test "bom tool should work inside run" {
  skip_slow_test
  cat > stacker.yaml <<EOF
bom-parent:
    from:
        type: oci
        url: $CENTOS_OCI
    bom:
      generate: true
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
      /stacker/tools/static-stacker bom discover
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

bom-child:
  from:
    type: built
    tag: bom-parent
  bom:
    generate: ${{GENERATE}}
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
    stacker build --substitute GENERATE=true
    [ -f .stacker/artifacts/bom-parent/installed-packages.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/bom-parent/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/bom-parent/bom-parent.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/bom-child/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/bom-child/bom-child.json ]
    if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
      zot_setup
      stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --tag latest
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/bom-parent:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/bom-parent:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/bom-child:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/bom-child:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
      zot_teardown
    fi
    stacker clean
}

@test "bom for alpine-based image" {
  skip_slow_test
  cat > stacker.yaml <<EOF
bom-alpine:
  from:
    type: docker
    url: docker://ghcr.io/project-stacker/alpine:edge
  bom:
    generate: ${{GENERATE}}
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
    [ ${{GENERATE}} = true ] && /stacker/tools/static-stacker bom discover
    # run our cmds
    ls -al  /
    # some changes
    touch /file1
    # cleanup
    rm -f /etc/alpine-release /etc/apk/arch /etc/apk/repositories /etc/apk/world /etc/issue /etc/os-release /etc/secfixes.d/alpine /lib/apk/db/installed /lib/apk/db/lock /lib/apk/db/scripts.tar /lib/apk/db/triggers
EOF
    stacker build --substitute GENERATE=true
    [ -f .stacker/artifacts/bom-alpine/installed-packages.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/bom-alpine/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/bom-alpine/bom-alpine.json ]
    if [ -n "${ZOT_HOST}:${ZOT_PORT}" ]; then
      zot_setup
      stacker publish --skip-tls --url docker://${ZOT_HOST}:${ZOT_PORT} --tag latest
      refs=$(regctl artifact tree ${ZOT_HOST}:${ZOT_PORT}/bom-alpine:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs -eq 2 ]
      refs=$(regctl artifact get --subject ${ZOT_HOST}:${ZOT_PORT}/bom-alpine:latest --filter-artifact-type "application/spdx+json" | jq '.SPDXID')
      [ $refs == \"SPDXRef-DOCUMENT\" ]
      zot_teardown
    fi
    stacker clean
}
