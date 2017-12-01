.PHONY: default
default:
	go get -v ./...
	go install -v ./...

# For now, let's just leave the binaries in $GOPATH/bin, but we can at least
# make stackermount suid. Note that we leave group as the user group, so that
# we can still delete/overwrite the file for rebuilds. Also note that go build
# (for now at least) truncates the file instead of removing and readding it, so
# the perms stay and you only need to run install once.
.PHONY: install
install: default
	sudo chown root:$(USER) $(shell which stackermount)
	sudo chmod 4755 $(shell which stackermount)

.PHONY: check
check:
	go fmt ./... && git diff --quiet
	go test ./...
