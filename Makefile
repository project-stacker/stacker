GO_SRC=$(shell find . -path ./.build -prune -false -o -name \*.go)
VERSION=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))

LXC_VERSION?=$(shell pkg-config --modversion lxc)

BUILD_TAGS = exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo static_build

STACKER_OPTS=--oci-dir=.build/oci --roots-dir=.build/roots --stacker-dir=.build/stacker --storage-type=overlay

build_stacker = go build -tags "$(BUILD_TAGS)" -ldflags "-X main.version=$(VERSION_FULL) -X main.lxc_version=$(LXC_VERSION) $1" -o $2 ./cmd

STACKER_BUILD_BASE_IMAGE?=docker://alpine:edge
LXC_CLONE_URL?=https://github.com/tych0/lxc
LXC_BRANCH?=4.0-fix-cgroup-warning

stacker: stacker-dynamic
	./stacker-dynamic --debug $(STACKER_OPTS) build \
		-f build.yaml --shell-fail \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH)

stacker-static: $(GO_SRC) go.mod go.sum cmd/lxc-wrapper/lxc-wrapper
	$(call build_stacker,-extldflags '-static',stacker)

# TODO: because we clean lxc-wrapper in the nested build, this always rebuilds.
# Could find a better way to do this.
stacker-dynamic: $(GO_SRC) go.mod go.sum cmd/lxc-wrapper/lxc-wrapper
	$(call build_stacker,,stacker-dynamic)

cmd/lxc-wrapper/lxc-wrapper: cmd/lxc-wrapper/lxc-wrapper.c
	make -C cmd/lxc-wrapper lxc-wrapper

.PHONY: lint
lint: cmd/lxc-wrapper/lxc-wrapper $(GO_SRC)
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
	sudo -E PATH="$$PATH" LXC_BRANCH="$(LXC_BRANCH)" ./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(shell [ -z $(STORAGE_TYPE) ] || echo --storage-type=$(STORAGE_TYPE)) \
		$(patsubst %,test/%.bats,$(TEST))

.PHONY: vendorup
vendorup:
	go get -u
	go mod tidy

.PHONY: clean
clean:
	./stacker-dynamic $(STACKER_OPTS) clean
	-rm -rf stacker stacker-dynamic .build
	-rm -r ./test/centos ./test/ubuntu
	-make -C cmd/lxc-wrapper clean
