## Building and Installing Stacker

### Go Dependency

Stacker requires at least go 1.20.

#### Ubuntu 22.04

On Ubuntu 22.04 you can install Go using the instructions at:
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

The other build dependencies can be satisfied by running:

#### **Ubuntu 22.04**


    This script will install the current library and build-time depedencies.
    Once installed it will prepare the system for build by fetching golang
    tools, downloading go modules and preparing a mirror of remote OCI images.


    sudo ./install-build-deps.sh

**To run `make check` you will also need:**

**umoci** - https://github.com/opencontainers/umoci

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
simple `make stacker`. The stacker binary will be output as `./stacker`.
