GO_SRC=$(shell find . -name \*.go)
COMMIT_HASH=$(shell git rev-parse HEAD)
COMMIT=$(if $(shell git status --porcelain --untracked-files=no),$(COMMIT_HASH)-dirty,$(COMMIT_HASH))

default: vendor $(GO_SRC)
	go build -ldflags "-X main.version=$(COMMIT)" -o $(GOPATH)/bin/stacker github.com/anuvu/stacker/stacker

vendor: glide.lock
	glide install --strip-vendor

.PHONY: check
check:
	go fmt ./... && git diff --quiet
	go test ./...

.PHONY: vendorup
vendorup:
	glide up --strip-vendor

clean:
	-rm -r vendor
