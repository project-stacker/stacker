### Building

Stacker requires go 1.9, on Ubuntu you can get that with:

    sudo apt-add-repository ppa:gophers/archive
    sudo apt update
    sudo apt install golang-1.9
    export PATH=$PATH:/usr/lib/go-1.9/bin

And uses glide for dependency management, which unfortunately is installed with
curl piped to sh as follows:

    curl https://glide.sh/get | sh

Stacker also has the following build dependencies:

    sudo apt install lxc-dev libacl1-dev libgpgme-dev

Finally, once you have the build dependencies, stacker can be built with a
simple `make`. The stacker binary will be output to `$GOPATH/bin/stacker`.
