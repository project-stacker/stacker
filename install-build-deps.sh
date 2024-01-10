#!/bin/bash
set -o pipefail
set -o errexit

installdeps_fedora() {
  sudo dnf install \
    jq \
    lxc-devel \
    libcap-devel \
    libacl-devel
  # skopeo deps
  sudo dnf install \
    gpgme-devel \
    libassuan-devel \
    btrfs-progs-devel \
    device-mapper-devel
    if ! command -v go 2>/dev/null; then
        sudo dnf install golang
        go version
    fi
}

installdeps_ubuntu() {
    sudo add-apt-repository -y ppa:project-machine/squashfuse
    sudo apt -yy install \
            build-essential \
            cryptsetup-bin \
            jq \
            libacl1-dev \
            libcap-dev \
            libcryptsetup-dev \
            libdevmapper-dev \
            libpam0g-dev \
            libseccomp-dev \
            libselinux1-dev \
            libssl-dev \
            libzstd-dev \
            lxc-dev \
            lxc-utils \
            parallel \
            pkg-config \
            squashfs-tools \
            squashfuse
    # skopeo deps
    sudo apt -yy install \
       libgpgme-dev \
       libassuan-dev \
       libbtrfs-dev \
       libdevmapper-dev \
       pkg-config
    if ! command -v go 2>/dev/null; then
        sudo apt -yy install golang-go
        go version
    fi
    # Work around an Ubuntu packaging bug. Fixed in 23.04 onward.
    sudo sed -i 's/#define LXC_DEVEL 1/#define LXC_DEVEL 0/' /usr/include/lxc/version.h
}

installdeps_golang() {
    go version
    GO111MODULE=off go get github.com/opencontainers/umoci/cmd/umoci
    make download-tools
    make docker-clone
    make go-download
}

. /etc/os-release

# install platform deps
case $ID_LIKE in
    debian|ubuntu) installdeps_ubuntu;;
    redhat|fedora) installdeps_fedora;;
    *)
        echo "Unknown os ID_LIKE value $ID_LIKE"
        exit 1
        ;;
esac

# install golang deps
installdeps_golang || exit 1
