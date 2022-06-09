## Building and Installing Stacker

### Go Dependency

Stacker requires at least go 1.11.

#### Ubuntu 20.04

On Ubuntu 20.04 you can install Go using the instructions at:
https://github.com/golang/go/wiki/Ubuntu

#### Fedora 31

On Fedora 31 you can install Go with the following command:

    sudo dnf install golang

#### Other Distributions

If Go is not already packaged for your Linux distribution, you can get the
latest Go version here:
https://golang.org/dl/#stable

Go can be installed using the instructions on on the official Go website:
https://golang.org/doc/install#install

### Other Dependencies

#### **Ubuntu 20.04**

The other build dependencies can be satisfied with the following command and
packages:

    sudo apt install lxc-dev libacl1-dev libgpgme-dev libcap-dev libseccomp-dev
    sudo apt install libpam0g-dev libselinux-dev libssl-dev libzstd-dev libcryptsetup-dev libdevmapper-dev

#### **Ubuntu 22.04**

    sudo apt install lxc-dev libacl1-dev libgpgme-dev libcap-dev libseccomp-dev
    sudo apt install libpam0g-dev libselinux-dev libssl-dev libzstd-dev libcryptsetup-dev libdevmapper-dev cryptsetup-bin pkg-config libsquashfs1 libsquashfs-dev


**To run `make check` you will also need:**

    sudo apt install bats jq tree

**umoci** - https://github.com/opencontainers/umoci

**squashtool**, but with a slightly different config than what is mentioned in the install guide (see below) - https://github.com/anuvu/squashfs

Contrary to what the documentation in squashfs implies, squashtool and
libsquash from squash-tools-ng need to be installed globally, as user specific
path overrides aren't propagated into `make check`'s test envs.

Thus, when you reach the step **install into mylocal="$HOME/lib"** from the squashfs guide, use the config below. You can put them at the end of your .bashrc file so you don't need to run them every time.

    mylocal="/usr/local"
    export LD_LIBRARY_PATH=$mylocal/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}
    export PKG_CONFIG_PATH=$mylocal/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}

Since the path **/usr/local** is owned by root, when you reach the step to run **make install**, you need to run it as **sudo**.

`make check`  requires the **golangci-lint** binary to be present in $GOPATH/bin

Since there are some tests that run with elevated privileges and use git, it will complain that the stacker folder is unsafe as it is owned by your user. To prevent that, we need to tell git to consider that folder as safe. To do this, open your git config file (**.gitconfig**) and add the following line with the path to your local stacker folder. Below is an example:

    [safe]
        directory = /home/chofnar/github/stacker


#### **Fedora 31**

The other build dependencies can be satisfied with the following command and
packages:

    sudo dnf install lxc-devel libcap-devel libacl-devel gpgme-devel
    sudo dnf install bats jq

### Building the Stacker Binary

Finally, once you have the build dependencies, stacker can be built with a
simple `make`. The stacker binary will be output as `./stacker`.
