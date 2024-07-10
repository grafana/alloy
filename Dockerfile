# syntax=docker/dockerfile:1.4

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.3 as build
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

# Build the UI before building Alloy, which will then bake the final UI into
# the binary.
RUN --mount=type=cache,target=/src/alloy/web/ui/node_modules,sharing=locked \
   make generate-ui

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    RELEASE_BUILD=${RELEASE_BUILD} VERSION=${VERSION} \
    GO_TAGS="netgo builtinassets promtail_journal_enabled" \
    GOEXPERIMENT=${GOEXPERIMENT} \
    make alloy

FROM public.ecr.aws/ubuntu/ubuntu:mantic

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
