### Building

Stacker requires go 1.10, on Ubuntu 18.04 you can get that with:

    sudo apt install golang

If you are on another Ubuntu release you can use the gophers' ppa:

    sudo apt-add-repository ppa:gophers/archive
    sudo apt update
    sudo apt install golang-1.10
    export PATH=$PATH:/usr/lib/go-1.10/bin

And uses glide for dependency management, which unfortunately is installed with
curl piped to sh as follows:

    curl https://glide.sh/get | sh

Stacker also has the following build dependencies:

    sudo apt install lxc-dev libacl1-dev libgpgme-dev libcap-dev

To run `make check` you will also need:

    sudo add-apt-repository ppa:projectatomic/ppa
    sudo apt update
    sudo apt install skopeo
    sudo apt install bats jq

Finally, once you have the build dependencies, stacker can be built with a
simple `make`. The stacker binary will be output to `$GOPATH/bin/stacker`.
