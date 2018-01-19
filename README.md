# stacker [![Build Status](https://travis-ci.org/anuvu/stacker.svg?branch=master)](https://travis-ci.org/anuvu/stacker)

Stacker is a tool for building OCI images via a declarative yaml format.

### Hacking

Stacker requires go 1.9, on Ubuntu you can get that with:

    sudo apt-add-repository ppa:gophers/archive
    sudo apt update
    sudo apt install golang-1.9
    export PATH=$PATH:/usr/lib/go-1.9/bin

The test suite requires `jq`, which can be installed on Ubuntu:

    sudo apt install jq

And `umoci`, which can be installed with:

    go get github.com/openSUSE/umoci/cmd/umoci

And can be run with

    cd test
    sudo -E ./main.sh

It will exit 0 on failure. There are several environment variables available:

1. `STACKER_KEEP` keeps the base layers and files, so you don't need to keep
   downloading them
1. `STACKER_INSPECT` stops the test suite before cleanup, so you can inspect
   the failure

### Install

You'll need both the stacker and stackermount binaries:

    go install github.com/anuvu/stacker/stacker
    go install github.com/anuvu/stacker/stackermount

Stacker also depends on skopeo for some operations; you can install skopeo on
ubuntu with:

    sudo apt-add-repository ppa:projectatomic/ppa
    sudo apt update
    sudo apt install skopeo

## Example

An example recipe file would look like:

```yaml
centos:
    from:
        type: tar
        url: http://example.com/centos.tar.gz
    environment:
        http_proxy: http://example.com:8080
        https_proxy: https://example.com:8080
boot:
    from:
        type: built
        tag: centos
    run: |
        yum install openssh-server
        echo meshuggah rocks
web:
    from:
        type: built
        tag: centos
    import: ./lighttp.cfg
    run: |
        yum install lighttpd
        cp /stacker/lighttp.cfg /etc/lighttpd/lighttp.cfg
```

If the above contents are in ./stacker.yaml, then the result of running

```bash
stacker
```

would be an OCI image with three images: centos, boot, and web.

See the manpage for more information.
