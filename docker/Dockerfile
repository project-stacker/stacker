FROM golang:alpine
RUN apk update && apk upgrade
RUN apk add build-base curl git lxc-dev acl-dev btrfs-progs btrfs-progs-dev \
    gpgme-dev glib-dev libassuan-dev lvm2-dev bash
# Install these additional packages from edge. These packages are required by
# skopeo.
RUN apk add ostree-dev libselinux-dev --update-cache \
    --repository http://dl-3.alpinelinux.org/alpine/edge/testing/

RUN curl -s https://glide.sh/get | sh
RUN go get github.com/openSUSE/umoci/cmd/umoci
RUN go get github.com/cpuguy83/go-md2man

# skopeo build will fail if we use /bin/sh as the shell.
SHELL ["/bin/bash", "-c"]
RUN git clone https://github.com/projectatomic/skopeo \
    $GOPATH/src/github.com/projectatomic/skopeo && \
    cd $GOPATH/src/github.com/projectatomic/skopeo && \
    make binary-local && make install

SHELL ["/bin/sh", "-c"]
RUN git clone https://github.com/anuvu/stacker.git \
    $GOPATH/src/github.com/anuvu/stacker && \
    cd $GOPATH/src/github.com/anuvu/stacker && \
    make
