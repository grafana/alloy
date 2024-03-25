# Grafana Agent build images

The Grafana Agent build images are used for CI workflows to manage builds of
Grafana Agent.

There are two images:

* `grafana/agent-build-image:X.Y.Z` (for building Linux containers)
* `grafana/agent-build-image:X.Y.Z-windows` (for building Windows containers)

(Where `X.Y.Z` is replaced with some semantic version, like 0.14.0).

## Pushing new images

Once a commit is merged to main which updates the build-image Dockerfiles, a
maintainer must push a tag matching the pattern `build-image/vX.Y.Z` to the
grafana/agent repo. For example, to create version v0.15.0 of the build images,
a maintainer would push the tag `build-image/v0.15.0`.

> **NOTE**: The tag name is expected to be prefixed with `v`, but the pushed
> images have the v prefix removed.

Automation will trigger off of this tag being pushed, building and pushing the
new build images to Docker Hub.

A follow-up commit to use the newly pushed build images must be made.

