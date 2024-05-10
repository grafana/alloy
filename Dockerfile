# syntax=docker/dockerfile:1.4

# NOTE: This Dockerfile can only be built using BuildKit. BuildKit is used by
# default when running `docker buildx build` or when DOCKER_BUILDKIT=1 is set
# in environment variables.

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.0 as build
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


RUN ./build-linux-amd64 alloy

FROM public.ecr.aws/ubuntu/ubuntu:mantic

# Username and uid for alloy user
ARG UID=473
ARG USERNAME="alloy"

LABEL org.opencontainers.image.source="https://github.com/grafana/alloy"

# Install dependencies needed at runtime.
RUN  apt-get update \
 &&  apt-get install -qy libsystemd-dev tzdata ca-certificates \
 &&  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*


COPY --from=build /src/alloy/build/alloy /bin/alloy
COPY example-config.alloy /etc/alloy/config.alloy

# Create alloy user in container, but do not set it as default
RUN groupadd --gid $UID $USERNAME
RUN useradd -m -u $UID -g $UID $USERNAME
RUN chown -R $USERNAME:$USERNAME /etc/alloy
RUN chown -R $USERNAME:$USERNAME /bin/alloy

RUN mkdir -p /var/lib/alloy/data
RUN chown -R $USERNAME:$USERNAME /var/lib/alloy
RUN chmod -R 770 /var/lib/alloy

ENTRYPOINT ["/bin/alloy"]
ENV ALLOY_DEPLOY_MODE=docker
CMD ["run", "/etc/alloy/config.alloy", "--storage.path=/var/lib/alloy/data"]
