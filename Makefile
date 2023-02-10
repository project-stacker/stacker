GO_SRC=$(shell find . -path ./.build -prune -false -o -name \*.go)
VERSION?=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL?=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))

LXC_VERSION?=$(shell pkg-config --modversion lxc)

BUILD_TAGS = exclude_graphdriver_btrfs exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo

STACKER_OPTS=--oci-dir=.build/oci --roots-dir=.build/roots --stacker-dir=.build/stacker --storage-type=overlay

build_stacker = go build $1 -tags "$(BUILD_TAGS) $2" -ldflags "-X main.version=$(VERSION_FULL) -X main.lxc_version=$(LXC_VERSION) $3" -o $4 ./cmd/stacker

STACKER_DOCKER_BASE?=docker://
STACKER_BUILD_BASE_IMAGE?=$(STACKER_DOCKER_BASE)alpine:edge
STACKER_BUILD_CENTOS_IMAGE?=$(STACKER_DOCKER_BASE)centos:latest
STACKER_BUILD_UBUNTU_IMAGE?=$(STACKER_DOCKER_BASE)ubuntu:latest
LXC_CLONE_URL?=https://github.com/lxc/lxc
LXC_BRANCH?=stable-5.0

stacker: stacker-dynamic
	./stacker-dynamic --debug $(STACKER_OPTS) build \
		-f build.yaml --shell-fail \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION_FULL=$(VERSION_FULL) \
		--substitute WITH_COV=no

stacker-cov: stacker-dynamic
	./stacker-dynamic --debug $(STACKER_OPTS) build \
		-f build.yaml --shell-fail \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION_FULL=$(VERSION_FULL) \
		--substitute WITH_COV=yes

stacker-static: $(GO_SRC) go.mod go.sum cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,,static_build,-extldflags '-static',stacker)

# can't use a comma in func call args, so do this instead
, := ,
stacker-static-cov: $(GO_SRC) go.mod go.sum cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,-cover -coverpkg="./pkg/...$(,)./cmd/...",static_build,-extldflags '-static',stacker)

# TODO: because we clean lxc-wrapper in the nested build, this always rebuilds.
# Could find a better way to do this.
stacker-dynamic: $(GO_SRC) go.mod go.sum cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,,,,stacker-dynamic)

cmd/stacker/lxc-wrapper/lxc-wrapper: cmd/stacker/lxc-wrapper/lxc-wrapper.c
	make -C cmd/stacker/lxc-wrapper LDFLAGS=-static LDLIBS="$(shell pkg-config --static --libs lxc) -lpthread -ldl" lxc-wrapper

.PHONY: lint
lint: cmd/stacker/lxc-wrapper/lxc-wrapper $(GO_SRC)
	go mod tidy
	go fmt ./... && ([ -z $(CI) ] || git diff --exit-code)
	bash test/static-analysis.sh
	go test -tags "$(BUILD_TAGS)" ./...
	$(shell go env GOPATH)/bin/golangci-lint run --build-tags "$(BUILD_TAGS)"

TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))
PRIVILEGE_LEVEL?=

# make check TEST=basic will run only the basic test
# make check PRIVILEGE_LEVEL=unpriv will run only unprivileged tests
.PHONY: check
check: stacker lint
	sudo -E PATH="$$PATH" \
		LXC_BRANCH=$(LXC_BRANCH) \
		LXC_CLONE_URL=$(LXC_CLONE_URL) \
		STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		STACKER_BUILD_CENTOS_IMAGE=$(STACKER_BUILD_CENTOS_IMAGE) \
		STACKER_BUILD_UBUNTU_IMAGE=$(STACKER_BUILD_UBUNTU_IMAGE) \
		./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(patsubst %,test/%.bats,$(TEST))

check-cov: stacker-cov lint
	sudo -E PATH="$$PATH" \
		LXC_BRANCH=$(LXC_BRANCH) \
		LXC_CLONE_URL=$(LXC_CLONE_URL) \
		STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		STACKER_BUILD_CENTOS_IMAGE=$(STACKER_BUILD_CENTOS_IMAGE) \
		STACKER_BUILD_UBUNTU_IMAGE=$(STACKER_BUILD_UBUNTU_IMAGE) \
		GOCOVERDIR=. \
		./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(patsubst %,test/%.bats,$(TEST))

.PHONY: vendorup
vendorup:
	go get -u
	go mod tidy

.PHONY: clean
clean:
	-unshare -Urm rm -rf stacker stacker-dynamic .build
	-rm -r ./test/centos ./test/ubuntu
	-make -C cmd/stacker/lxc-wrapper clean
