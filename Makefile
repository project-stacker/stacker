GO_SRC=$(shell find . -name \*.go)
COMMIT_HASH=$(shell git rev-parse HEAD)
COMMIT=$(if $(shell git status --porcelain --untracked-files=no),$(COMMIT_HASH)-dirty,$(COMMIT_HASH))

# Note: we don't really need stackermount right now since we're only
# privileged, so we intentionally avoid building it.
default: vendor $(GO_SRC)
	go build -ldflags "-X main.version=$(COMMIT)" -o $(GOPATH)/bin/stacker github.com/anuvu/stacker/stacker

# For now, let's just leave the binaries in $GOPATH/bin, but we can at least
# make stackermount suid. Note that we leave group as the user group, so that
# we can still delete/overwrite the file for rebuilds. Also note that go build
# (for now at least) truncates the file instead of removing and readding it, so
# the perms stay and you only need to run install once.
.PHONY: install
install: default
	sudo chown root:$(USER) $(shell which stackermount)
	sudo chmod 4755 $(shell which stackermount)

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
