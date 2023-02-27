load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "all container contents must be accounted for" {
  cat > stacker.yaml <<EOF
bom-test:
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
      /stacker-bom discover -o /stacker-artifacts/installed-packages.json
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
    stacker clean
}

@test "bom tool should work inside run" {
  cat > stacker.yaml <<EOF
bom-test:
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
      /stacker-bom discover -o /stacker-artifacts/installed-packages.json
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
      org.opencontainers.image.authors: bom-test
      org.opencontainers.image.vendor: bom-test
      org.opencontainers.image.licenses: MIT
EOF
    stacker build
    [ -f .stacker/artifacts/centos/installed-packages.json ]
    # sbom for this image
    [ -f .stacker/artifacts/centos/centos.json ]
    # a full inventory for this image
    [ -f .stacker/artifacts/centos/inventory.json ]
    stacker clean
}

