## Building and Installing Stacker

### Go Dependency

Stacker requires at least go 1.11.

#### Ubuntu 18.04

On Ubuntu 18.04 you can install Go using the instructions at:
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

#### Ubuntu 18.04

The other build dependencies can be satisfied with the following command and
packages:

    sudo apt install lxc-dev libacl1-dev libgpgme-dev libcap-dev

To run `make check` you will also need:

    sudo add-apt-repository ppa:projectatomic/ppa
    sudo apt update
    sudo apt install bats jq

#### Fedora 31

The other build dependencies can be satisfied with the following command and
packages:

    sudo dnf install lxc-devel libcap-devel libacl-devel gpgme-devel
    sudo dnf install btrfs-progs
    sudo dnf install bats jq

### Building the Stacker Binary

Finally, once you have the build dependencies, stacker can be built with a
simple `make`. The stacker binary will be output as `./stacker`.
