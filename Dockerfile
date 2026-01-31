# syntax=docker/dockerfile:1.21@sha256:27f9262d43452075f3c410287a2c43f5ef1bf7ec2bb06e8c9eeb1b8d453087bc

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.26@sha256:54ef2371fc6b049d932c58d4622c88b573b45c393b7622fe6cca492ac4525dbe AS ui-build
ARG BUILDPLATFORM
COPY ./internal/web/ui /ui
WORKDIR /ui
RUN --mount=type=cache,target=/ui/node_modules,sharing=locked \
    npm install                                               \
    && npm run build

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.26@sha256:54ef2371fc6b049d932c58d4622c88b573b45c393b7622fe6cca492ac4525dbe AS build

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

COPY --from=ui-build /ui/dist /src/alloy/internal/web/ui/dist

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS="$TARGETOS" GOARCH="$TARGETARCH" GOARM=${TARGETVARIANT#v} \
    RELEASE_BUILD=${RELEASE_BUILD} VERSION=${VERSION} \
    GO_TAGS="netgo builtinassets promtail_journal_enabled" \
    GOEXPERIMENT=${GOEXPERIMENT} \
    make alloy

###

FROM public.ecr.aws/ubuntu/ubuntu:noble@sha256:6b4c0f97bc73e76ac20ef992258e9b8c831b7755d2047d83109da8eb279881fe

# Username and uid for alloy user
ARG UID="473"
ARG USERNAME="alloy"
# Force non-interactive mode for tzdata package install
ARG DEBIAN_FRONTEND="noninteractive"

LABEL org.opencontainers.image.source="https://github.com/grafana/alloy"

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -qy --no-install-recommends \
        ca-certificates \
        libsystemd0 \
        tzdata \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*


COPY --from=build --chown=${UID}:${UID} /src/alloy/build/alloy /bin/alloy
COPY --chown=${UID}:${UID} example-config.alloy /etc/alloy/config.alloy

# Create alloy user in container, but do not set it as default
#
# NOTE: non-root support in Docker containers is an experimental,
# undocumented feature; use at your own risk.
RUN groupadd --gid $UID $USERNAME \
    && useradd -m -u $UID -g $UID $USERNAME \
    && mkdir -p /var/lib/alloy/data \
    && chown -R $USERNAME:$USERNAME /var/lib/alloy \
    && chmod -R 770 /var/lib/alloy

ENTRYPOINT ["/bin/alloy"]
ENV ALLOY_DEPLOY_MODE=docker
CMD ["run", "/etc/alloy/config.alloy", "--storage.path=/var/lib/alloy/data"]
