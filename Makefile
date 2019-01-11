GO_SRC=$(shell find . -name \*.go)
COMMIT_HASH=$(shell git rev-parse HEAD)
COMMIT=$(if $(shell git status --porcelain --untracked-files=no),$(COMMIT_HASH)-dirty,$(COMMIT_HASH))
TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))

default: vendor $(GO_SRC)
	go build -ldflags "-X main.version=$(COMMIT)" -o $(GOPATH)/bin/stacker github.com/anuvu/stacker/stacker

vendor: glide.lock
	glide install --strip-vendor

# make test TEST=basic will run only the basic test.
.PHONY: check
check:
	go fmt ./... && ([ -z $(TRAVIS) ] || git diff --quiet)
	go test ./...
	sudo -E "PATH=$$PATH" bats -t $(patsubst %,test/%.bats,$(TEST))

.PHONY: vendorup
vendorup:
	glide cc
	glide up --strip-vendor

clean:
	-rm -r vendor
