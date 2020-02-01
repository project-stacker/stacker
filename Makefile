GO_SRC=$(shell find . -name \*.go)
VERSION=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))
TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))

stacker: $(GO_SRC)
	go build -ldflags "-X main.version=$(VERSION_FULL)" -o stacker ./cmd

# make check TEST=basic will run only the basic test.
.PHONY: check
check: stacker
	go fmt ./... && ([ -z $(TRAVIS) ] || git diff --quiet)
	go test -tags "exclude_graphdriver_devicemapper" ./...
	sudo -E "PATH=$$PATH" bats -t $(patsubst %,test/%.bats,$(TEST))

.PHONY: vendorup
vendorup:
	go get -u

.PHONY: clean
clean:
	-rm -r stacker
