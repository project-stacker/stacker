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
    sudo dnf install bsdtar
}

COMMON_DEBS=(
	apache2-utils
	build-essential
	cryptsetup-bin
	curl
	erofsfuse
	erofs-utils
	golang-go
	jq
	libacl1-dev
	libarchive-tools
	libassuan-dev
	libbtrfs-dev
	libcap-dev
	libcryptsetup-dev
	libdevmapper-dev
	libgpgme-dev
	libpam0g-dev
	libseccomp-dev
	libselinux1-dev
	libssl-dev
	libzstd-dev
	lxc-dev
	parallel
	pkg-config
	psmisc
	shellcheck
	squashfs-tools
	squashfuse
	uidmap
	uuid-runtime
	zip
)

SKOPEO_DEBS=(
    libgpgme-dev
    libassuan-dev
    libbtrfs-dev
    libdevmapper-dev
    pkg-config
)

installdeps_ubuntu() {
    PKGS=(
        liblxc-dev
        lxc-utils
    )

    case "$VERSION_ID" in
        22.04)
            sudo add-apt-repository -y ppa:project-machine/squashfuse
            ;;
        24.04)
            # lp:2080069
            # temporarily add puzzleos/dev to pickup lxc-dev package which
            # provides static liblxc.a
            sudo add-apt-repository -y ppa:puzzleos/dev-next

            # allow array to expand again
            #shellcheck disable=2206
            PKGS=( ${PKGS[*]} libsystemd-dev )

            # 24.04 has additional apparmor restrictions, probably doesn't apply
            # for root in github VM but developers will run into this
            enable_userns
            ;;
    esac

    sudo apt update
    echo "Checking lxc-dev policy after possible ppa install"
    sudo apt-cache policy lxc-dev

    # allow array to expand
    #shellcheck disable=2206,2048,2086
    sudo apt -yy install ${COMMON_DEBS[*]} ${PKGS[*]}

    # Work around an Ubuntu packaging bug. Fixed in 23.04 onward.
    if [ "$VERSION_ID" != "24.04" ]; then
        sudo sed -i 's/#define LXC_DEVEL 1/#define LXC_DEVEL 0/' /usr/include/lxc/version.h
    fi

    # skopeo requirements
    # allow array to expand
    #shellcheck disable=2206,2048,2086
    sudo apt -yy install ${SKOPEO_DEBS[*]}
    if ! command -v go 2>/dev/null; then
        sudo apt -yy install golang-go
        go version
    fi

    # cloud kernels, like linux-azure, don't include erofs in the linux-modules package and instead put it linux-modules-extra
    if ! sudo modinfo erofs &>/dev/null; then
        sudo apt -yy install "linux-modules-extra-$(uname -r)"
    fi
}

installdeps_debian() {
    PKGS=(
        lxc-dev
        zlib1g-dev
    )

    # allow array to expand
    #shellcheck disable=2206,2048,2086
    sudo apt-get -yy install ${COMMON_DEBS[*]} ${PKGS[*]}

    # skopeo requirements
    # allow array to expand
    #shellcheck disable=2206,2048,2086
    sudo apt -yy install ${SKOPEO_DEBS[*]}
    if ! command -v go 2>/dev/null; then
        sudo apt -yy install golang-go
        go version
    fi

    # make sure we have erofs kernel module
    if ! sudo modinfo erofs &>/dev/null; then
        echo "missing erofs kernel module; please install correct linux kernel package with erorfs module"
        exit 1
    fi

    case "$VERSION_ID" in
        13) enable_userns;;
    esac
}

enable_userns() {
    SYSCTL_USERNS="/etc/sysctl.d/00-enable-userns.conf"
    if ! [ -s "${SYSCTL_USERNS}" ]; then
        echo "Add kernel tunables to enable user namespaces in $SYSCTL_USERNS "
        cat <<EOF | sudo tee "${SYSCTL_USERNS}"
kernel.apparmor_restrict_unprivileged_io_uring = 0
kernel.apparmor_restrict_unprivileged_unconfined = 0
kernel.apparmor_restrict_unprivileged_userns = 0
kernel.apparmor_restrict_unprivileged_userns_complain = 0
kernel.apparmor_restrict_unprivileged_userns_force = 0
kernel.unprivileged_bpf_disabled = 2
kernel.unprivileged_userns_apparmor_policy = 0
kernel.unprivileged_userns_clone = 1
EOF
        sudo sysctl -p /etc/sysctl.d/00-enable-userns.conf
    fi
}

installdeps_golang() {
    go version
    make download-tools
    make docker-clone
    make go-download
}

. /etc/os-release
lsb_release -rd

# install platform deps
case $ID in
    ubuntu) installdeps_ubuntu;;
    debian) installdeps_debian;;
    redhat|fedora) installdeps_fedora;;
    *)
        echo "Unknown os ID_LIKE value $ID_LIKE"
        exit 1
        ;;
esac

# add container policy (if not already present
POLICY="/etc/containers/policy.json"
if ! [ -s "${POLICY}" ]; then
    sudo mkdir -p "$(dirname $POLICY)"
    echo "adding default containers policy (insecure):${POLICY}"
    echo '{"default":[{"type":"insecureAcceptAnything"}]}' | sudo tee "${POLICY}"
fi

# install golang deps
installdeps_golang || exit 1
