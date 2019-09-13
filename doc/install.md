### Building

Stacker requires at least go 1.11, since it uses go modules.
On Ubuntu 18.04 you can get that using the instructions at:
https://github.com/golang/go/wiki/Ubuntu

Or you could get the latest version available at:
https://golang.org/dl/#stable
And install it using the instructions on on the official go website:
https://golang.org/doc/install#install

Stacker also has the following build dependencies:

    sudo apt install lxc-dev libacl1-dev libgpgme-dev libcap-dev

To run `make check` you will also need:

    sudo add-apt-repository ppa:projectatomic/ppa
    sudo apt update
    sudo apt install skopeo
    sudo apt install bats jq

Finally, once you have the build dependencies, stacker can be built with a
simple `make`. The stacker binary will be output as `./stacker`.
