# syntax=docker/dockerfile:1.21@sha256:27f9262d43452075f3c410287a2c43f5ef1bf7ec2bb06e8c9eeb1b8d453087bc

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.28@sha256:982eb27dfc3111a3a81eb605d2e80d0c6c21a32842c2652b6390be6cd80da6fb AS ui-build
ARG BUILDPLATFORM
COPY ./internal/web/ui /ui
WORKDIR /ui
RUN --mount=type=cache,target=/ui/node_modules,sharing=locked \
    npm install                                               \
    && npm run build

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.28@sha256:982eb27dfc3111a3a81eb605d2e80d0c6c21a32842c2652b6390be6cd80da6fb AS build

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

FROM public.ecr.aws/ubuntu/ubuntu:noble@sha256:b1940c8ecf8ff591053cc5db0303fb882f9fafec50f26892a870bcbe1b30d25a

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
