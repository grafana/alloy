# syntax=docker/dockerfile:1.23@sha256:2780b5c3bab67f1f76c781860de469442999ed1a0d7992a5efdf2cffc0e3d769

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

# NOTE: GO_RUNTIME pins the Go toolchain image that provides the bundled Go and
# Go-based tools. It's supplied by the build-image workflow; there's no in-tree default.
ARG GO_RUNTIME=mustoverride

#
# Dependencies
#
# We retrieve many of our dependencies by using various smaller containers.
#

# Dependency: docker (for building images)
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 as docker
RUN apk add --no-cache docker-cli docker-cli-buildx

# Dependency: helm
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 as helm
RUN apk add --no-cache helm

# Dependency: Go and Go dependencies
FROM ${GO_RUNTIME} as golang

ENV CONTROLLER_GEN_VERSION v0.9.2

RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@$CONTROLLER_GEN_VERSION \
 && go install github.com/tcnksm/ghr@v0.15.0                                           \
 && go install github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb@v0.5.1                \
 && go install github.com/google/go-jsonnet/cmd/jsonnet@v0.18.0                        \
 && go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.11.0

#
# Final image
#

# rfratto/viceroy contains C cross compilers can be used for our Cgo
# dependencies.
FROM rfratto/viceroy:v0.4.0@sha256:7e3f328889b5b0c5c0303dda5e74a89189e6e437d33f609dde98f9a84337e0a2

# Fix apt sources - restrict main bullseye to supported architectures
RUN rm -f /etc/apt/sources.list \
 && rm -rf /etc/apt/sources.list.d/* \
 && echo "deb [arch=amd64,arm64,armhf,i386] https://ftp.debian.org/debian bullseye main" > /etc/apt/sources.list \
 && echo "deb https://ftp.debian.org/debian bullseye-updates main" >> /etc/apt/sources.list \
 && echo "deb https://security.debian.org/debian-security bullseye-security main" >> /etc/apt/sources.list

# Install other dependencies.
#
# NOTE(rfratto): musl is installed so the Docker binaries from alpine work
# properly.
RUN apt-get update                                \
 && apt-get install -qy                           \
      build-essential file zip unzip gettext git  \
      jq musl libsystemd-dev nsis                 \
      rpm ruby ruby-dev rubygems yamllint         \
 && gem install --no-document fpm                 \
 && rm -rf /var/lib/apt/lists/*

COPY --from=docker   /usr/bin/docker                     /usr/bin/docker
COPY --from=docker   /usr/libexec/docker/cli-plugins     /usr/libexec/docker/cli-plugins
COPY --from=helm     /usr/bin/helm                       /usr/bin/helm
COPY --from=golang   /usr/local/go                       /usr/local/go
COPY --from=golang   /go/bin                             /go/bin

# Install Node.js 20 directly from NodeSource repository
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
  && apt-get install -y nodejs \
  && corepack enable \
  && node -v \
  && npm -v

# Install GitHub CLI (https://github.com/cli/cli/blob/trunk/docs/install_linux.md#debian)
RUN (type -p wget >/dev/null || (apt update && apt install wget -y)) \
  && mkdir -p -m 755 /etc/apt/keyrings \
  && out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  && cat $out | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
  && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
  && mkdir -p -m 755 /etc/apt/sources.list.d \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
  && apt update \
  && apt install gh -y

# Git tries to prevent misuse of repositories (CVE-2022-24765), but we don't
# care about this for build containers, where it's expected that the repository
# will be accessed by other users (the root user of the build container).
#
# Disable that safety check.
RUN git config --global --add safe.directory \*

# Set CC to viceroycc to ensure that the cross compilers are used for all C
# compilation.
ENV CC viceroycc

ENV GOPATH /go
ENV PATH /usr/local/go/bin:/go/bin:$PATH
