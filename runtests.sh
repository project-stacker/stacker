#!/usr/bin/env sh

rm -f works.log broken.log

rm -f /tmp/use-uniq-container-name-test
make test TEST=empty-layers >& works.log
touch  /tmp/use-uniq-container-name-test
make test TEST=empty-layers >& broken.log

delta works.log broken.log
