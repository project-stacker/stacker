### Testing

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
