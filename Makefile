SHELL=/bin/bash
TOP_LEVEL := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
BUILD_D = $(TOP_LEVEL)/.build
export GOPATH ?= $(BUILD_D)/gopath
export GOCACHE ?= $(GOPATH)/gocache

GO_SRC=$(shell find pkg cmd -name \*.go)
GOARCH=$(shell go env GOARCH)
GOOS=$(shell go env GOOS)
# --tags includes both annotated and lightweight tags but must match our expected versioning
VERSION?=$(shell git describe --tags --always --long --dirty --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || echo 'no-git')
BUILD_ID?=$(shell git rev-parse --short HEAD || echo 'no-git')
HASH = \#

LXC_VERSION?=$(shell pkg-config --modversion lxc)

BUILD_TAGS = exclude_graphdriver_btrfs exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo

STACKER_OPTS=--oci-dir=$(BUILD_D)/oci --roots-dir=$(BUILD_D)/roots --stacker-dir=$(BUILD_D)/stacker --storage-type=overlay

VERSION_LDFLAGS=-X stackerbuild.io/stacker/pkg/lib.StackerVersion=$(VERSION) -X stackerbuild.io/stacker/pkg/lib.LXCVersion=$(LXC_VERSION)
build_stacker = go build $1 -tags "$(BUILD_TAGS) $2" -ldflags "$(VERSION_LDFLAGS) $3" -o $4 ./cmd/stacker

# See doc/hacking.md for how to use a local oci or docker repository.
STACKER_DOCKER_BASE?=docker://ghcr.io/project-stacker/
# They default to their image name in STACKER_DOCKER_BASE
STACKER_BUILD_BASE_IMAGE?=$(STACKER_BUILD_ALPINE_IMAGE)
STACKER_BUILD_ALPINE_IMAGE?=$(STACKER_DOCKER_BASE)alpine:3.19
STACKER_BUILD_BUSYBOX_IMAGE?=$(STACKER_DOCKER_BASE)busybox:latest
STACKER_BUILD_CENTOS_IMAGE?=$(STACKER_DOCKER_BASE)centos:latest
STACKER_BUILD_UBUNTU_IMAGE?=$(STACKER_DOCKER_BASE)ubuntu:latest
STACKER_BUILD_IMAGES = \
	$(STACKER_BUILD_ALPINE_IMAGE) \
	$(STACKER_BUILD_BASE_IMAGE) \
	$(STACKER_BUILD_BUSYBOX_IMAGE) \
	$(STACKER_BUILD_CENTOS_IMAGE) \
	$(STACKER_BUILD_UBUNTU_IMAGE)

LXC_CLONE_URL?=https://github.com/lxc/lxc
LXC_BRANCH?=stable-5.0

HACK_D := $(TOP_LEVEL)/hack
# helper tools
TOOLS_D := $(HACK_D)/tools
REGCLIENT := $(TOOLS_D)/bin/regctl
REGCLIENT_VERSION := v0.5.1
SKOPEO = $(TOOLS_D)/bin/skopeo
export SKOPEO_VERSION = 1.13.0
BATS = $(TOOLS_D)/bin/bats
BATS_VERSION := v1.13.0
BATS_VERSION_STAMP := $(TOOLS_D)/.bats-$(BATS_VERSION)
# OCI registry
ZOT := $(TOOLS_D)/bin/zot
ZOT_VERSION := v2.1.8
UMOCI := $(TOOLS_D)/bin/umoci
UMOCI_VERSION := main

export PATH := $(TOOLS_D)/bin:$(PATH)

GOLANGCI_LINT_VERSION = 2.7.2
GOLANGCI_LINT_URL = https://github.com/golangci/golangci-lint/releases/download
GOLANGCI_LINT = $(TOOLS_D)/bin/golangci-lint

STAGE1_STACKER ?= ./stacker-dynamic
STACKER_PUBLISH_BIN = stacker-$(GOOS)-$(GOARCH)
LXC_WRAPPER_DYNAMIC = cmd/stacker/lxc-wrapper/lxc-wrapper-host
LXC_WRAPPER_STATIC = cmd/stacker/lxc-wrapper/lxc-wrapper-static
LINT = $(BUILD_D)/lint
GO_TEST = $(BUILD_D)/go-test

STACKER_DEPS = $(GO_SRC) go.mod go.sum
STACKER_DYNAMIC_DEPS = $(GO_TEST) $(STACKER_DEPS) $(LXC_WRAPPER_DYNAMIC)
STACKER_STATIC_DEPS = $(STACKER_DEPS) $(LXC_WRAPPER_STATIC)

stacker: $(STAGE1_STACKER) build.yaml
	echo STACKER_DOCKER_BASE=$(STACKER_DOCKER_BASE)
	echo STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE)
	$(STAGE1_STACKER) --debug $(STACKER_OPTS) build \
		-f build.yaml \
		--substitute BUILD_D=$(BUILD_D) \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION=$(VERSION) \
		--substitute WITH_COV=no

stacker-cov: $(STAGE1_STACKER) build.yaml
	$(STAGE1_STACKER) --debug $(STACKER_OPTS) build \
		-f build.yaml \
		--substitute BUILD_D=$(BUILD_D) \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION=$(VERSION) \
		--substitute WITH_COV=yes

.PHONY: publish-stacker-bin
publish-stacker-bin: $(STACKER_PUBLISH_BIN)

$(STACKER_PUBLISH_BIN): stacker
	cp -v $< $@

# On Ubuntu 24.04 the lxc package does not link against libsystemd so the pkg-config
# below does list -lsystemd; we must add it to the list but only for stacker-dynamic
OS_VERSION_ID ?= "24.04"
ifneq (,$(wildcard /etc/os-release))
OS_VERSION_ID := $(shell awk -F= '/VERSION_ID/ {print $$2}' /etc/os-release)
endif
ifeq ($(OS_VERSION_ID),"24.04")
ifeq (stacker-dynamic,$(firstword $(MAKECMDGOALS)))
LXC_WRAPPER_LIBS=-lsystemd
else
LXC_WRAPPER_LIBS=
endif
endif

stacker-static: $(STACKER_STATIC_DEPS)
	$(call build_stacker,,static_build,-extldflags '-static',stacker)

# can't use a comma in func call args, so do this instead
, := ,
stacker-static-cov: $(STACKER_STATIC_DEPS)
	$(call build_stacker,-cover -coverpkg="./pkg/...$(,)./cmd/...",static_build,-extldflags '-static',stacker)

stacker-dynamic: $(STACKER_DYNAMIC_DEPS)
	$(call build_stacker,,,,stacker-dynamic)

$(LXC_WRAPPER_DYNAMIC) $(LXC_WRAPPER_STATIC): cmd/stacker/lxc-wrapper/lxc-wrapper.c
	make -C cmd/stacker/lxc-wrapper OUTPUT=$(notdir $@) LDFLAGS=-static LDLIBS="$(shell pkg-config --static --libs lxc) $(LXC_WRAPPER_LIBS) -lpthread -ldl"


.PHONY: go-download
go-download:
	go mod download

lint: $(LINT)

$(LINT): $(GO_SRC) go.mod go.sum $(GOLANGCI_LINT)
	go mod tidy
	go fmt ./... && ([ -z $(CI) ] || git diff --exit-code)
	bash test/static-analysis.sh
	$(GOLANGCI_LINT) run --build-tags "$(BUILD_TAGS) skipembed"
	@mkdir -p $(dir $@)
	@touch $@

go-test: $(GO_TEST)

$(GO_TEST): $(LINT) $(GO_SRC) go.mod go.sum
	go test -v -trimpath -cover -coverprofile=coverage.txt -covermode=atomic -tags "exclude_graphdriver_btrfs exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo skipembed" ./pkg/... ./cmd/...
	go tool cover -html coverage.txt  -o $(HACK_D)/coverage.html
	@mkdir -p $(dir $@)
	@touch $@

.PHONY: download-tools
download-tools: $(GOLANGCI_LINT) $(REGCLIENT) $(ZOT) $(BATS) $(UMOCI) $(SKOPEO)

$(GOLANGCI_LINT):
	@[ -x $(GOLANGCI_LINT) ] || \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) ..." && \
		mkdir -p "$(TOOLS_D)/bin" && \
		curl -sSfL $(GOLANGCI_LINT_URL)/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-linux-$(GOARCH).tar.gz | \
		tar -C $(TOOLS_D)/bin -xzf - --strip-components=1 golangci-lint-$(GOLANGCI_LINT_VERSION)-linux-$(GOARCH)/golangci-lint
	@$(GOLANGCI_LINT) version

# dlbin is used with $(call dlbin,path,url)
# it downloads a url to path and makes it executable.
# it creates dest dir and atomically moves into place. t gets <name>.pid
dlbin = set -x; mkdir -p $(dir $1) && t=$1.$$$$ && curl -Lo "$$t" "$2" && chmod +x "$$t" && mv "$$t" "$1"

$(REGCLIENT):
	$(call dlbin,$@,https://github.com/regclient/regclient/releases/download/$(REGCLIENT_VERSION)/regctl-linux-$(GOARCH))

$(ZOT):
	$(call dlbin,$@,https://github.com/project-zot/zot/releases/download/$(ZOT_VERSION)/zot-linux-$(GOARCH)-minimal)

$(SKOPEO):
	@set -e; mkdir -p "$(TOOLS_D)/bin"; \
	tmpdir=$$(mktemp -d); \
	cd $$tmpdir; \
	git clone https://github.com/containers/skopeo.git; \
	cd skopeo; \
	git fetch --all --tags --prune; \
	git checkout tags/v$(SKOPEO_VERSION) -b tag-$(SKOPEO_VERSION); \
	make bin/skopeo; \
	cp bin/skopeo $(SKOPEO); \
	cd $(TOP_LEVEL); \
	rm -rf $$tmpdir;

$(BATS): $(BATS_VERSION_STAMP)

$(BATS_VERSION_STAMP):
	mkdir -p $(TOOLS_D)/bin
	rm -rf bats-core
	git clone -b $(BATS_VERSION) https://github.com/bats-core/bats-core.git
	cd bats-core; ./install.sh $(TOOLS_D) ; rm -rf bats-core
	rm -rf $(TOP_LEVEL)/test/test_helper
	mkdir -p $(TOP_LEVEL)/test/test_helper
	git clone --depth 1 https://github.com/bats-core/bats-support $(TOP_LEVEL)/test/test_helper/bats-support
	git clone --depth 1 https://github.com/bats-core/bats-assert $(TOP_LEVEL)/test/test_helper/bats-assert
	git clone --depth 1 https://github.com/bats-core/bats-file $(TOP_LEVEL)/test/test_helper/bats-file
	touch $@


$(UMOCI):
	rm -rf ${GOPATH}/src/github.com/opencontainers/
	mkdir -p ${GOPATH}/src/github.com/opencontainers/
	git clone https://github.com/opencontainers/umoci.git ${GOPATH}/src/github.com/opencontainers/umoci
	cd ${GOPATH}/src/github.com/opencontainers/umoci ; git reset --hard ${UMOCI_VERSION} ; make umoci ; mv umoci $(UMOCI)
	$(UMOCI) --version

TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))
PRIVILEGE_LEVEL ?= unpriv

# make check TEST=basic will run only the basic test
# make check PRIVILEGE_LEVEL=unpriv will run only unprivileged tests
.PHONY: check
check:
	$(MAKE) go-test
	$(MAKE) test

.PHONY: test
test: stacker download-tools lintbats
	sudo -E PATH="$(PATH)" \
		STACKER_BUILD_ALPINE_IMAGE=$(STACKER_BUILD_ALPINE_IMAGE) \
		STACKER_BUILD_BUSYBOX_IMAGE=$(STACKER_BUILD_BUSYBOX_IMAGE) \
		STACKER_BUILD_CENTOS_IMAGE=$(STACKER_BUILD_CENTOS_IMAGE) \
		STACKER_BUILD_UBUNTU_IMAGE=$(STACKER_BUILD_UBUNTU_IMAGE) \
		TOP_LEVEL=$(TOP_LEVEL) \
		BUILD_ID=$(BUILD_ID) \
		VERSION=$(VERSION) \
		./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(patsubst %,test/%.bats,$(TEST))

.PHONY: lintbats
lintbats:
	# check only SC2031 which finds undefined variables in bats tests but is only an INFO
	shellcheck -i SC2031 $(patsubst %,test/%.bats,$(TEST))
	# check all error level issues
	shellcheck -S error  $(patsubst %,test/%.bats,$(TEST))

.PHONY: check-cov
check-cov: lint test-cov

.PHONY: test-cov
test-cov: stacker-cov download-tools
	sudo -E PATH="$(PATH)" \
		-E GOCOVERDIR="$$GOCOVERDIR" \
		STACKER_BUILD_ALPINE_IMAGE=$(STACKER_BUILD_ALPINE_IMAGE) \
		STACKER_BUILD_BUSYBOX_IMAGE=$(STACKER_BUILD_BUSYBOX_IMAGE) \
		STACKER_BUILD_CENTOS_IMAGE=$(STACKER_BUILD_CENTOS_IMAGE) \
		STACKER_BUILD_UBUNTU_IMAGE=$(STACKER_BUILD_UBUNTU_IMAGE) \
		TOP_LEVEL=$(TOP_LEVEL) \
		BUILD_ID=$(BUILD_ID) \
		VERSION=$(VERSION) \
		./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(patsubst %,test/%.bats,$(TEST))

CLONE_D ?= "$(BUILD_D)/oci-clone"

.PHONY: docker-clone
docker-clone: $(SKOPEO)
	./tools/oci-copy $(CLONE_D) $(STACKER_BUILD_IMAGES)

.PHONY: show-info
show-info:
	@echo BUILD_D=$(BUILD_D)
	@echo BUILD_ID=$(BUILD_ID)
	@echo VERSION=$(VERSION)
	@echo TOP_LEVEL=$(TOP_LEVEL)
	@go env

.PHONY: vendorup
vendorup:
	go get -u
	go mod tidy

.PHONY: debug
debug:
	@echo TOP_LEVEL=$(TOP_LEVEL)
	@echo BUILD_ID=$(BUILD_ID)
	@echo VERSION=$(VERSION)

.PHONY: clean
clean:
	-unshare -Urm rm -rf stacker stacker-dynamic .build
	-rm -rf ./test/centos ./test/ubuntu ./test/busybox ./test/alpine ./test/test_helper
	-make -C cmd/stacker/lxc-wrapper clean
	-rm -rf $(TOOLS_D)
