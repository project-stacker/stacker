GO_SRC=$(shell find . -name \*.go)
VERSION=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))
TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))

BUILD_TAGS = exclude_graphdriver_devicemapper containers_image_openpgp

stacker: $(GO_SRC) go.mod go.sum
	go build -tags "$(BUILD_TAGS)" -ldflags "-X main.version=$(VERSION_FULL)" -o stacker ./cmd

# make check TEST=basic will run only the basic test.
.PHONY: check
check: stacker
	go fmt ./... && ([ -z $(TRAVIS) ] || git diff --quiet)
	go test -tags "$(BUILD_TAGS)" ./...
	$(shell go env GOPATH)/bin/golangci-lint run --build-tags "$(BUILD_TAGS)"
	sudo -E "PATH=$$PATH" bats -t $(patsubst %,test/%.bats,$(TEST))

.PHONY: vendorup
vendorup:
	go get -u

.PHONY: clean
clean:
	-rm -r stacker
