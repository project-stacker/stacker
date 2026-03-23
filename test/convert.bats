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
  # https://raw.githubusercontent.com/alpinelinux/docker-alpine/refs/heads/master/Dockerfile
  cat > Dockerfile << EOF
FROM alpine:3.16
RUN apk add --no-cache lua5.3 lua-filesystem lua-lyaml lua-http
COPY fetch-latest-releases.lua /usr/local/bin
VOLUME /out
ENTRYPOINT [ "/usr/local/bin/fetch-latest-releases.lua" ]
EOF
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
  # https://github.com/elastic/dockerfiles/archive/refs/tags/v8.17.10.tar.gz
  # SHA256SUM=75f89195b53c9d02d6a45d2bc9d51a11d92b079dcc046893615b04455269ba1a  dockerfiles-8.17.10/elasticsearch/Dockerfile
  DOCKERFILE="data/elasticsearch-8.17.10.Dockerfile"
  stacker convert --docker-file $DOCKERFILE --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=elasticsearch
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=elasticsearch --skip-tls --url docker://${REGISTRY_URL} --image elasticsearch --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}
@test "python" {
  skip_slow_test
  # git clone https://github.com/docker-library/python.git
  # cd python
  # pick a specific commit so we don't get broken by upstream changes:
  # git reset --hard aad39d215779f27b410b25f612b6680a75781edb
  # cd 3.11/alpine3.22
  # SHA256SUM=ada7a604a8359de5914158bca13f539ea24c13d8a3492f511a68444a511a1db8  data/python-3.11-alpine-3.22-Dockerfile
  DOCKERFILE="data/python-3.11-alpine-3.22-Dockerfile"
  stacker convert --docker-file $DOCKERFILE --output-file stacker.yaml --substitute-file stacker-subs.yaml
  stacker build -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=python
  if [ -nz "${REGISTRY_URL}" ]; then
    stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=python --skip-tls --url docker://${REGISTRY_URL} --image python --tag latest
  fi
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}
