GO_SRC=$(shell find . -name \*.go)
VERSION=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))

BUILD_TAGS = exclude_graphdriver_devicemapper containers_image_openpgp

stacker: $(GO_SRC) go.mod go.sum lxc-wrapper/lxc-wrapper
	go build -tags "$(BUILD_TAGS)" -ldflags "-X main.version=$(VERSION_FULL)" -o stacker ./cmd

lxc-wrapper/lxc-wrapper: lxc-wrapper/lxc-wrapper.c
	make -C lxc-wrapper lxc-wrapper

.PHONY: lint
lint: lxc-wrapper/lxc-wrapper $(GO_SRC)
	go mod tidy
	go fmt ./... && ([ -z $(CI) ] || git diff --exit-code)
	bash test/static-analysis.sh
	go test -tags "$(BUILD_TAGS)" ./...
	$(shell go env GOPATH)/bin/golangci-lint run --build-tags "$(BUILD_TAGS)"

TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))
PRIVILEGE_LEVEL?=
STORAGE_TYPE?=

# make check TEST=basic will run only the basic test
# make check PRIVILEGE_LEVEL=unpriv will run only unprivileged tests
# make check STORAGE_TYPE=btrfs will run only btrfs tests
.PHONY: check
check: stacker lint
	sudo -E PATH="$$PATH" ./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(shell [ -z $(STORAGE_TYPE) ] || echo --storage-type=$(STORAGE_TYPE)) \
		$(patsubst %,test/%.bats,$(TEST))

.PHONY: vendorup
vendorup:
	go get -u
	go mod tidy

.PHONY: clean
clean:
	-rm -r stacker
	-rm -r ./test/centos ./test/ubuntu
	-make -C lxc-wrapper clean
