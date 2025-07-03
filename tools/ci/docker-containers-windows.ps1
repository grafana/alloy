    docker build                           \
      -t "$DEVEL_ALLOY_IMAGE:$VERSION_TAG" \
      -t "$DEVEL_ALLOY_IMAGE:$BRANCH_TAG"  \
      --build-arg VERSION="$VERSION"       \
      --build-arg RELEASE_BUILD=1          \
      --build-arg BASE_IMAGE_WINDOWS="$BASE_IMAGE_WINDOWS" \
      -f ./Dockerfile.windows              \
      --isolation=hyperv                   \
