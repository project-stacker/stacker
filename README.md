# stacker [![Build Status](https://travis-ci.org/anuvu/stacker.svg?branch=master)](https://travis-ci.org/anuvu/stacker)

Stacker is a tool for building OCI images via a declarative yaml format.

### Hacking

The test suite can be run with

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
