load helpers

function setup_file() {
    start_registry
}

function teardown_file() {
    stop_registry
}

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
    rm -rf recursive bing.ico || true
}

@test "convert a Dockerfile" {
    cat > Dockerfile <<EOF
FROM public.ecr.aws/docker/library/alpine:edge
VOLUME /out
ARG VERSION=1.0.0
MAINTAINER unknown
ENV ENV_VERSION1 \$VERSION
ENV ENV_VERSION2=\$VERSION
ENV ENV_VERSION3=\$\{VERSION\}
ENV TEST_PATH="/usr/share/test/bin:$PATH" \
    TEST_PATHS_CONFIG="/etc/test/test.ini" \
    TEST_PATHS_DATA="/var/lib/test" \
    TEST_PATHS_HOME="/usr/share/test" \
    TEST_PATHS_LOGS="/var/log/test" \
    TEST_PATHS_PLUGINS="/var/lib/test/plugins" \
    TEST_PATHS_PROVISIONING="/etc/test/provisioning"
ENV COMMIT_SHA=${COMMIT_SHA}
RUN echo \$VERSION
RUN echo \$\{VERSION\}
RUN apk add --no-cache lua5.3 lua-filesystem lua-lyaml lua-http
ENTRYPOINT [ "/usr/local/bin/fetch-latest-releases.lua" ]
EOF
  # first convert
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  cat stacker.yaml
  cat stacker-subs.yaml
  # build should now work
  ## docker build -t test
  mkdir -p /out
  echo "# building converted Dockerfile; this may take several minutes" >&3
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=app
  if [ -z "${REGISTRY_URL}" ]; then
    skip "publish step of test because no registry found in REGISTRY_URL env variable"
  fi
  stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=app --skip-tls --url docker://${REGISTRY_URL} --image app --tag latest
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}

@test "alpine convert and build" {
  skip_slow_test
  echo "# cloning alpine Dockerfile repository; this may take some time" >&3
  git clone --depth 1 https://github.com/alpinelinux/docker-alpine.git
  chmod -R a+rwx docker-alpine
  cd docker-alpine
  TEMPDIR=$(mktemp -d)
  echo "# converting and building alpine; this may take several minutes" >&3
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=alpine --substitute STACKER_VOL1="$TEMPDIR"
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=alpine --substitute STACKER_VOL1="$TEMPDIR" --skip-tls --url docker://${REGISTRY_URL} --image alpine --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}

@test "elasticsearch convert and build" {
  skip_slow_test
  echo "# cloning elasticsearch Dockerfile repository; this may take some time" >&3
  git clone --branch v8.17.10 --depth 1 https://github.com/elastic/dockerfiles.git
  chmod -R a+rwx dockerfiles
  cd dockerfiles/elasticsearch
  echo "# converting and building elasticsearch; this may take several minutes" >&3
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=elasticsearch
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=elasticsearch --skip-tls --url docker://${REGISTRY_URL} --image elasticsearch --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}
@test "python3 convert and build" {
  skip_slow_test
  echo "# cloning python Dockerfile repository; this may take some time" >&3
  git init -q python
  cd python
  git remote add origin https://github.com/docker-library/python.git
  # Fetch only the pinned commit so we do not need the repository's full history.
  git fetch --depth 1 origin aad39d215779f27b410b25f612b6680a75781edb
  git checkout --detach FETCH_HEAD
  cd 3.11/alpine3.22
  chmod -R a+rw .
  echo "# converting and building python; this may take several minutes" >&3
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=python
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=python --skip-tls --url docker://${REGISTRY_URL} --image python --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}

@test "convert FROM-AS in Dockerfile" {
  # Remove any prior Dockerfile
  rm -f Dockerfile

  # Create myhello.go
  cat > myhello.go << EOF
package main

func main() {
}
EOF

  # Create a Dockerfile
  cat > Dockerfile << EOF
FROM golang AS mybuild
COPY myhello.go /src/myhello.go
WORKDIR /src
RUN export GOPATH=/go && export PATH=/go/bin:/usr/local/go/bin:\$PATH && export HOME=/go && go build -o /bin/myhello myhello.go && ls /bin/myhello
FROM alpine AS mybase
FROM mybase AS A
COPY --from=mybuild /bin/myhello /bin/myhello
FROM A AS B
RUN chmod 755 /bin/myhello && \
    /bin/myhello
EOF

  # Convert
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml

  cat stacker.yaml

  # Ensure fields are correctly converted
  grep -A 5 "A:" stacker.yaml | grep -zo "tag: mybase.*type: built"
  grep -A 5 "B:" stacker.yaml | grep -zo "tag: A.*type: built"
  grep -A 5 "mybase:" stacker.yaml | grep -zo "type: docker.*url: docker://alpine"
  grep -A 5 "mybuild:" stacker.yaml | grep -zo "type: docker.*url: docker://golang"

  # build should work
  stacker build -f stacker.yaml --substitute IMAGE=testFROMAS

  rm -f stacker.yaml stacker-subs.yaml Dockerfile myhello.go
  stacker clean
}
