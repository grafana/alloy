# Grafana Alloy build images

The Grafana Alloy build images are used for CI workflows to manage builds of
Grafana Alloy.

There are the following images:

* `grafana/alloy-build-image:vX.Y.Z` (for building Linux & Windows containers)
* `grafana/alloy-build-image:vX.Y.Z-boringcrypto` (for building Linux containers with boringcrypto)

(Where `X.Y.Z` is replaced with some semantic version, like 0.1.0).

## Pushing new images

Once a commit is merged to main which updates the build-image Dockerfiles, a
maintainer must push a tag matching the pattern `build-image/vX.Y.Z` to the
grafana/alloy repo. For example, to create version v0.1.0 of the build images,
a maintainer would push the tag `build-image/v0.1.0`.

Automation will trigger off of this tag being pushed, building and pushing the
new build images to Docker Hub.

A follow-up commit to use the newly pushed build images must be made.
