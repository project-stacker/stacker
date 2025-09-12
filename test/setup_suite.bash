#!/bin/bash

function write_certs {
    pushd $BATS_SUITE_TMPDIR

    openssl req \
        -newkey rsa:2048 \
        -nodes \
        -days 3650 \
        -x509 \
        -keyout ca.key \
        -out ca.crt \
        -subj "/CN=*"

    openssl req \
        -newkey rsa:2048 \
        -nodes \
        -keyout server.key \
        -out server.csr \
        -subj "/OU=TestServer/CN=*"

    openssl x509 \
        -req \
        -days 3650 \
        -sha256 \
        -in server.csr \
        -CA ca.crt \
        -CAkey ca.key \
        -CAcreateserial \
        -out server.cert \
        -extfile <(echo subjectAltName = DNS:localhost)

    openssl req \
        -newkey rsa:2048 \
        -nodes \
        -keyout client.key \
        -out client.csr \
        -subj "/OU=TestClient/CN=*"

    openssl x509 \
        -req \
        -days 3650 \
        -sha256 \
        -in client.csr \
        -CA ca.crt \
        -CAkey ca.key \
        -CAcreateserial \
        -out client.cert
    popd
}

function setup_suite {

   write_certs

   if [ "$PRIVILEGE_LEVEL" = "priv" ]; then
      return
   fi
   if [ -z "$SUDO_UID" ]; then
      echo "setup_suite found PRIVILEGE_LEVEL=$PRIVILEGE_LEVEL but empty SUDO_USER"
      exit 1
   fi
   if [ -z "$BATS_RUN_TMPDIR" ]; then
       echo "setup_suite found empty BATS_RUN_TMPDIR"
       exit 1
   fi
   chmod ugo+x "${BATS_RUN_TMPDIR}"
}
