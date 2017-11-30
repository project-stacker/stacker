#!/usr/bin/env bash

PATH=$PATH:$GOPATH/bin

stacker build -f ./basic.yaml
