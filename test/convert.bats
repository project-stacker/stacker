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
FROM alpine:3.16
VOLUME /out
ARG VERSION=1.0.0
MAINTAINER unknown
ENV ENV_VERSION1 \$VERSION
ENV ENV_VERSION2=\$VERSION
ENV ENV_VERSION3=\$\{VERSION\}
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
    skip "skipping test because no registry found in REGISTRY_URL env variable"
  fi
  stacker publish -f stacker.yaml --substitute-file stacker-subs.yaml --substitute IMAGE=app --skip-tls --url docker://${REGISTRY_URL} --layer app --tag latest
  rm -f stacker.yaml stacker-subs.yaml
  stacker clean
}
