TOP_LEVEL := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
BUILD_D = $(TOP_LEVEL)/.build
export GOPATH = $(BUILD_D)/gopath
export GOCACHE = $(GOPATH)/gocache

GO_SRC=$(shell find pkg cmd -name \*.go)
VERSION?=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL?=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))

LXC_VERSION?=$(shell pkg-config --modversion lxc)

BUILD_TAGS = exclude_graphdriver_btrfs exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo

STACKER_OPTS=--oci-dir=$(BUILD_D)/oci --roots-dir=$(BUILD_D)/roots --stacker-dir=$(BUILD_D)/stacker --storage-type=overlay

build_stacker = go build -tags "$(BUILD_TAGS) $1" -ldflags "-X main.version=$(VERSION_FULL) -X main.lxc_version=$(LXC_VERSION) $2" -o $3 ./cmd/stacker

STACKER_DOCKER_BASE?=docker://
STACKER_BUILD_BASE_IMAGE?=$(STACKER_DOCKER_BASE)alpine:edge
STACKER_BUILD_CENTOS_IMAGE?=$(STACKER_DOCKER_BASE)centos:latest
STACKER_BUILD_UBUNTU_IMAGE?=$(STACKER_DOCKER_BASE)ubuntu:latest
LXC_CLONE_URL?=https://github.com/lxc/lxc
LXC_BRANCH?=stable-5.0

# helper tools
TOOLSDIR := $(shell pwd)/hack/tools
REGCLIENT := $(TOOLSDIR)/bin/regctl
REGCLIENT_VERSION := v0.5.1
# OCI registry
ZOT := $(TOOLSDIR)/bin/zot
ZOT_VERSION := 2.0.0-rc6

STAGE1_STACKER ?= ./stacker-dynamic

STACKER_DEPS = $(GO_SRC) go.mod go.sum

stacker: $(STAGE1_STACKER) $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper.c
	$(STAGE1_STACKER) --debug $(STACKER_OPTS) build \
		-f build.yaml --shell-fail \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION_FULL=$(VERSION_FULL)

stacker-static: $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,static_build,-extldflags '-static',stacker)

# TODO: because we clean lxc-wrapper in the nested build, this always rebuilds.
# Could find a better way to do this.
stacker-dynamic: $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,,,stacker-dynamic)

cmd/stacker/lxc-wrapper/lxc-wrapper: cmd/stacker/lxc-wrapper/lxc-wrapper.c
	make -C cmd/stacker/lxc-wrapper LDFLAGS=-static LDLIBS="$(shell pkg-config --static --libs lxc) -lpthread -ldl" lxc-wrapper


.PHONY: go-download
go-download:
	go mod download

.PHONY: lint
lint: cmd/stacker/lxc-wrapper/lxc-wrapper $(GO_SRC)
	go mod tidy
	go fmt ./... && ([ -z $(CI) ] || git diff --exit-code)
	bash test/static-analysis.sh
	go test -v -trimpath -cover -coverpkg stackerbuild.io/stacker/./... -coverprofile=coverage.txt -covermode=atomic -tags "$(BUILD_TAGS)" stackerbuild.io/stacker/./...
	$(shell go env GOPATH)/bin/golangci-lint run --build-tags "$(BUILD_TAGS)"

$(REGCLIENT):
	mkdir -p $(TOOLSDIR)/bin
	curl -Lo $(REGCLIENT) https://github.com/regclient/regclient/releases/download/$(REGCLIENT_VERSION)/regctl-linux-amd64
	chmod +x $(REGCLIENT)

$(ZOT):
	mkdir -p $(TOOLSDIR)/bin
	curl -Lo $(ZOT) https://github.com/project-zot/zot/releases/download/v$(ZOT_VERSION)/zot-linux-amd64-minimal
	chmod +x $(ZOT)

TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))
PRIVILEGE_LEVEL?=

# make check TEST=basic will run only the basic test
# make check PRIVILEGE_LEVEL=unpriv will run only unprivileged tests
.PHONY: check
check: stacker lint $(REGCLIENT) $(ZOT)
	sudo -E PATH="$$PATH" \
		LXC_BRANCH=$(LXC_BRANCH) \
		LXC_CLONE_URL=$(LXC_CLONE_URL) \
		STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		STACKER_BUILD_CENTOS_IMAGE=$(STACKER_BUILD_CENTOS_IMAGE) \
		STACKER_BUILD_UBUNTU_IMAGE=$(STACKER_BUILD_UBUNTU_IMAGE) \
		./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(patsubst %,test/%.bats,$(TEST))

.PHONY:
show-info:
	@echo BUILD_D=$(BUILD_D)
	@go env

.PHONY: vendorup
vendorup:
	go get -u
	go mod tidy

.PHONY: clean
clean:
	-unshare -Urm rm -rf stacker stacker-dynamic .build
	-rm -r ./test/centos ./test/ubuntu
	-make -C cmd/stacker/lxc-wrapper clean
