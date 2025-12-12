load helpers

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
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=app
  if [ -z "${REGISTRY_URL}" ]; then
    skip "publish step of test because no registry found in REGISTRY_URL env variable"
  fi
  stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=app --skip-tls --url docker://${REGISTRY_URL} --image app --tag latest
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}

@test "alpine" {
  skip_slow_test
  git clone https://github.com/alpinelinux/docker-alpine.git
  chmod -R a+rwx docker-alpine
  cd docker-alpine
  TEMPDIR=$(mktemp -d)
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=alpine --substitute STACKER_VOL1="$TEMPDIR"
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=alpine --substitute STACKER_VOL1="$TEMPDIR" --skip-tls --url docker://${REGISTRY_URL} --image alpine --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}

@test "elasticsearch" {
  skip_slow_test
  git clone https://github.com/elastic/dockerfiles.git
  chmod -R a+rwx dockerfiles
  cd dockerfiles/elasticsearch
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=elasticsearch
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=elasticsearch --skip-tls --url docker://${REGISTRY_URL} --image elasticsearch --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}

@test "python" {
  skip_slow_test
  git clone https://github.com/docker-library/python.git
  cd python
  # pick a specific commit so we don't get broken by upstream changes:
  git reset --hard aad39d215779f27b410b25f612b6680a75781edb
  cd 3.11/alpine3.22
  chmod -R a+rw .
  stacker convert --docker-file Dockerfile --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=python
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=python --skip-tls --url docker://${REGISTRY_URL} --image python --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}
