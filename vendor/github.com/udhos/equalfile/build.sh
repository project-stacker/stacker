#!/bin/sh

src=`find . -type f | egrep '\.go$'`

gofmt -s -w $src
go tool vet $src
go tool fix $src
go install github.com/udhos/equalfile
go install github.com/udhos/equalfile/equal

# go get honnef.co/go/simple/cmd/gosimple
s=$GOPATH/bin/gosimple
simple() {
    # gosimple cant handle source files from multiple packages
    $s *.go
    $s equal/*.go
}
[ -x "$s" ] && simple

# go get github.com/golang/lint/golint
l=$GOPATH/bin/golint
lint() {
    # golint cant handle source files from multiple packages
    $l *.go
    $l equal/*.go
}
[ -x "$l" ] && lint

go test -v
