# syntax=docker/dockerfile:1.24@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.33@sha256:dbdecbfbad6c9ed0b315a7a6da25ef066b795ffc57e887125333b1a352907155 AS ui-build
ARG BUILDPLATFORM
COPY ./internal/web/ui /ui
WORKDIR /ui
RUN --mount=type=cache,target=/ui/node_modules,sharing=locked \
    npm install                                               \
    && npm run build

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.33@sha256:dbdecbfbad6c9ed0b315a7a6da25ef066b795ffc57e887125333b1a352907155 AS build

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
    GO_TAGS="netgo embedalloyui promtail_journal_enabled" \
    GOEXPERIMENT=${GOEXPERIMENT} \
    SKIP_UI_BUILD=1 \
    make alloy

###

FROM public.ecr.aws/ubuntu/ubuntu:noble@sha256:8c10ecc59261c77dd866fa8587f1b9cbf172ad8f1253f0af96eaae0fa390c132

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

# Provide /bin/otelcol compatibility entrypoint. Useful when using Alloy's OTel Engine with
# OpenTelemetry Collector helm chart and other ecosystem tools that expect otelcol binary.
COPY packaging/docker/otelcol.sh /bin/otelcol
RUN chmod 755 /bin/otelcol

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
