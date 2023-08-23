load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "all container contents must be accounted for" {
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
EOF
    stacker build
    [ -f .stacker/artifacts/bom-parent/installed-packages.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/bom-parent/inventory.json ]
    # sbom for this image
    [ -f .stacker/artifacts/bom-parent/bom-parent.json ]
    if [ -nz "${REGISTRY_URL}" ]; then
      stacker publish --skip-tls --url docker://localhost:8080/ --tag latest
      refs=$(regctl artifact tree localhost:8080/bom-parent:latest --format "{{json .}}" | jq '.referrer | length')
      [ $refs eq 2 ]
      refs=$(regctl artifact tree localhost:8080/bom-parent:latest --filter-artifact-type "application/spdx+json" --format "{{json .}}" | jq '.SPDXID')
      [ $refs eq "SPDXRef-DOCUMENT" ]
    fi
    stacker clean
}

