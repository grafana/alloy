# syntax=docker/dockerfile:1.4

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.8 as pyroscope-build
ARG TARGETARCH
ARG RUST_TOOLCHAIN=1.77
RUN <<EOF
set -ex
apt-get update
apt-get install -y cmake
rustup || curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
  | sh -s -- -y --profile minimal --default-toolchain $RUST_TOOLCHAIN
/root/.cargo/bin/rustup target add aarch64-unknown-linux-musl
/root/.cargo/bin/rustup target add x86_64-unknown-linux-musl
EOF
ENV PATH $PATH:/root/.cargo/bin/
COPY go.mod Makefile ./
COPY tools/make/*mk  tools/make/
COPY tools/image-tag tools/image-tag-docker  tools/
RUN make pyroscope-dependencies \
        OTEL_PROFILER_ARCH=$TARGETARCH

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.8 as ui-build
COPY internal/web/ui internal/web/ui
COPY tools/make/*mk  tools/make/
COPY tools/image-tag tools/image-tag-docker  tools/
COPY Makefile ./
RUN --mount=type=cache,target=/internal/web/ui/node_modules,sharing=locked \
   make generate-ui

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.8 as build
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG RELEASE_BUILD=1
ARG VERSION
ARG GOEXPERIMENT

COPY . /src/alloy
WORKDIR /src/alloy

COPY --from=ui-build           ./internal/web/ui/build ./internal/web/ui/build
COPY --from=pyroscope-build    ./target                ./target

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    RELEASE_BUILD=${RELEASE_BUILD} VERSION=${VERSION} \
    GO_TAGS="netgo builtinassets promtail_journal_enabled pyroscope_ebpf" \
    GOEXPERIMENT=${GOEXPERIMENT} \
    make alloy

FROM public.ecr.aws/ubuntu/ubuntu:noble

# Username and uid for alloy user
ARG UID=473
ARG USERNAME="alloy"

LABEL org.opencontainers.image.source="https://github.com/grafana/alloy"

# Install dependencies needed at runtime.
RUN  apt-get update \
 &&  apt-get install -qy libsystemd-dev tzdata ca-certificates \
 &&  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*


COPY --from=build --chown=$UID /src/alloy/build/alloy /bin/alloy
COPY --chown=$UID example-config.alloy /etc/alloy/config.alloy

# Create alloy user in container, but do not set it as default
#
# NOTE(rfratto): non-root support in Docker containers is an experimental,
# undocumented feature; use at your own risk.
RUN groupadd --gid $UID $USERNAME
RUN useradd -m -u $UID -g $UID $USERNAME

RUN mkdir -p /var/lib/alloy/data
RUN chown -R $USERNAME:$USERNAME /var/lib/alloy
RUN chmod -R 770 /var/lib/alloy

ENTRYPOINT ["/bin/alloy"]
ENV ALLOY_DEPLOY_MODE=docker
CMD ["run", "/etc/alloy/config.alloy", "--storage.path=/var/lib/alloy/data"]
