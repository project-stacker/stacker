SHELL=/bin/bash
TOP_LEVEL := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
BUILD_D = $(TOP_LEVEL)/.build
export GOPATH ?= $(BUILD_D)/gopath
export GOCACHE ?= $(GOPATH)/gocache

GO_SRC=$(shell find pkg cmd -name \*.go)
VERSION?=$(shell git describe --tags || git rev-parse HEAD)
VERSION_FULL?=$(if $(shell git status --porcelain --untracked-files=no),$(VERSION)-dirty,$(VERSION))
HASH = \#

LXC_VERSION?=$(shell pkg-config --modversion lxc)

BUILD_TAGS = exclude_graphdriver_btrfs exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo

STACKER_OPTS=--oci-dir=$(BUILD_D)/oci --roots-dir=$(BUILD_D)/roots --stacker-dir=$(BUILD_D)/stacker --storage-type=overlay

VERSION_LDFLAGS=-X stackerbuild.io/stacker/pkg/lib.StackerVersion=$(VERSION_FULL) -X stackerbuild.io/stacker/pkg/lib.LXCVersion=$(LXC_VERSION)
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
BATS_VERSION := v1.10.0
# OCI registry
ZOT := $(TOOLS_D)/bin/zot
ZOT_VERSION := v2.1.0
UMOCI := $(TOOLS_D)/bin/umoci
UMOCI_VERSION := main

export PATH := $(TOOLS_D)/bin:$(PATH)

GOLANGCI_LINT_VERSION = v1.64.8
GOLANGCI_LINT = $(TOOLS_D)/golangci-lint/$(GOLANGCI_LINT_VERSION)/golangci-lint

STAGE1_STACKER ?= ./stacker-dynamic

STACKER_DEPS = $(GO_SRC) go.mod go.sum

stacker: $(STAGE1_STACKER) $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper.c
	$(STAGE1_STACKER) --debug $(STACKER_OPTS) build \
		-f build.yaml \
		--substitute BUILD_D=$(BUILD_D) \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION_FULL=$(VERSION_FULL) \
		--substitute WITH_COV=no

stacker-cov: $(STAGE1_STACKER) $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper.c
	$(STAGE1_STACKER) --debug $(STACKER_OPTS) build \
		-f build.yaml \
		--substitute BUILD_D=$(BUILD_D) \
		--substitute STACKER_BUILD_BASE_IMAGE=$(STACKER_BUILD_BASE_IMAGE) \
		--substitute LXC_CLONE_URL=$(LXC_CLONE_URL) \
		--substitute LXC_BRANCH=$(LXC_BRANCH) \
		--substitute VERSION_FULL=$(VERSION_FULL) \
		--substitute WITH_COV=yes

# On Ubuntu 24.04 the lxc package does not link against libsystemd so the pkg-config
# below does list -lsystemd; we must add it to the list but only for stacker-dynamic
ifeq ($(shell awk -F= '/VERSION_ID/ {print $$2}' /etc/os-release),"24.04")
ifeq (stacker-dynamic,$(firstword $(MAKECMDGOALS)))
LXC_WRAPPER_LIBS=-lsystemd
else
LXC_WRAPPER_LIBS=
endif
endif

stacker-static: $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,,static_build,-extldflags '-static',stacker)

# can't use a comma in func call args, so do this instead
, := ,
stacker-static-cov: $(GO_SRC) go.mod go.sum cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,-cover -coverpkg="./pkg/...$(,)./cmd/...",static_build,-extldflags '-static',stacker)

# TODO: because we clean lxc-wrapper in the nested build, this always rebuilds.
# Could find a better way to do this.
stacker-dynamic: $(STACKER_DEPS) cmd/stacker/lxc-wrapper/lxc-wrapper
	$(call build_stacker,,,,stacker-dynamic)

cmd/stacker/lxc-wrapper/lxc-wrapper: cmd/stacker/lxc-wrapper/lxc-wrapper.c
	make -C cmd/stacker/lxc-wrapper LDFLAGS=-static LDLIBS="$(shell pkg-config --static --libs lxc) $(LXC_WRAPPER_LIBS) -lpthread -ldl" lxc-wrapper


.PHONY: go-download
go-download:
	go mod download

.PHONY: lint
lint: $(GO_SRC) $(GOLANGCI_LINT)
	go mod tidy
	go fmt ./... && ([ -z $(CI) ] || git diff --exit-code)
	bash test/static-analysis.sh
	$(GOLANGCI_LINT) run --build-tags "$(BUILD_TAGS) skipembed"

.PHONY: go-test
go-test:
	go test -v -trimpath -cover -coverprofile=coverage.txt -covermode=atomic -tags "exclude_graphdriver_btrfs exclude_graphdriver_devicemapper containers_image_openpgp osusergo netgo skipembed" ./pkg/... ./cmd/...
	go tool cover -html coverage.txt  -o $(HACK_D)/coverage.html

.PHONY: download-tools
download-tools: $(GOLANGCI_LINT) $(REGCLIENT) $(ZOT) $(BATS) $(UMOCI)

$(GOLANGCI_LINT):
	@mkdir -p $(dir $@)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(dir $@)"
	@mkdir -p "$(TOOLS_D)/bin"
	ln -sf "$@" "$(TOOLS_D)/bin/"

# dlbin is used with $(call dlbin,path,url)
# it downloads a url to path and makes it executable.
# it creates dest dir and atomically moves into place. t gets <name>.pid
dlbin = set -x; mkdir -p $(dir $1) && t=$1.$$$$ && curl -Lo "$$t" "$2" && chmod +x "$$t" && mv "$$t" "$1"

$(REGCLIENT):
	$(call dlbin,$@,https://github.com/regclient/regclient/releases/download/$(REGCLIENT_VERSION)/regctl-linux-amd64)

$(ZOT):
	$(call dlbin,$@,https://github.com/project-zot/zot/releases/download/$(ZOT_VERSION)/zot-linux-amd64-minimal)

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

$(BATS):
	@set -e; rm -rf bats-core; \
	git clone -b $(BATS_VERSION) https://github.com/bats-core/bats-core.git; \
	cd bats-core; ./install.sh $(TOOLS_D); cd ..; \
	rm -rf bats-core

$(UMOCI):
	mkdir -p ${GOPATH}/src/github.com/opencontainers/
	git clone https://github.com/opencontainers/umoci.git ${GOPATH}/src/github.com/opencontainers/umoci
	cd ${GOPATH}/src/github.com/opencontainers/umoci ; git reset --hard ${UMOCI_VERSION} ; make umoci ; mv umoci $(UMOCI)
	$(UMOCI) --version

TEST?=$(patsubst test/%.bats,%,$(wildcard test/*.bats))
PRIVILEGE_LEVEL?=

# make check TEST=basic will run only the basic test
# make check PRIVILEGE_LEVEL=unpriv will run only unprivileged tests
.PHONY: check
check: lint test go-test

.PHONY: test
test: stacker download-tools lintbats
	sudo -E PATH="$(PATH)" \
		STACKER_BUILD_ALPINE_IMAGE=$(STACKER_BUILD_ALPINE_IMAGE) \
		STACKER_BUILD_BUSYBOX_IMAGE=$(STACKER_BUILD_BUSYBOX_IMAGE) \
		STACKER_BUILD_CENTOS_IMAGE=$(STACKER_BUILD_CENTOS_IMAGE) \
		STACKER_BUILD_UBUNTU_IMAGE=$(STACKER_BUILD_UBUNTU_IMAGE) \
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
		./test/main.py \
		$(shell [ -z $(PRIVILEGE_LEVEL) ] || echo --privilege-level=$(PRIVILEGE_LEVEL)) \
		$(patsubst %,test/%.bats,$(TEST))

.PHONY: docker-clone
docker-clone: $(SKOPEO)
	./tools/oci-copy "$(BUILD_D)/oci-clone" $(STACKER_BUILD_IMAGES)


.PHONY: show-info
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
