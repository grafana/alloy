#!/bin/sh
set -e -x

#DEV_VERSION_NAME="telegen-$(git rev-parse --short HEAD)"
DEV_VERSION_NAME="telegen-$(date +%Y%m%d%H%M%S)"
LOCAL_DEV_VERSION="$DEV_VERSION_NAME-dev"
REPONAME="piotrgrafana/telegen"

# Local dev platform build
docker build -t "$REPONAME:$LOCAL_DEV_VERSION" -f Dockerfile .
echo "Local dev version: $LOCAL_DEV_VERSION"


# Cloud build
DOCKER_BUILDKIT=1 docker buildx build --platform linux/amd64 --build-arg RELEASE_BUILD=0 --build-arg VERSION=$DEV_VERSION_NAME --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 -t $REPONAME:$DEV_VERSION_NAME -f Dockerfile .
docker push "$REPONAME:$DEV_VERSION_NAME"

echo "Successfully pushed!"
echo "Cloud version: $DEV_VERSION_NAME"
