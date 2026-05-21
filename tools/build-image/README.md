# Grafana Alloy build images

The Grafana Alloy build images are used for CI workflows to manage builds of Grafana Alloy
and/or to serve as base images for the official Alloy Docker containers.

There are several [build images][alloy-build-image-dockerhub]:

* `grafana/alloy-build-image:vX.Y.Z`: For building Linux containers.
* `grafana/alloy-build-image:vX.Y.Z-boringcrypto`: For building Linux containers with boringcrypto.
* `grafana/alloy-build-image:vX.Y.Z-windows`: Used as a base image for the official Alloy Windows Docker image. Not used for building Windows containers.

<!-- TODO (ptodev): Update the commend above after the GH Actions migration. 
     We will likely want to remove the Windows image entirely and not even use it for a base image. -->

Above, `X.Y.Z` is replaced with some semantic version like 0.1.0.

[alloy-build-image-dockerhub]:https://hub.docker.com/repository/docker/grafana/alloy-build-image/general

## Creating new images

### Step 1: Update the main branch

Open a PR to update the build images. 
See [this][example-pr] pull request for an example.

You need to change the following files:
 * `tools/build-image/Dockerfile`
 * `tools/build-image/windows/Dockerfile`

Also, search and replace the `.drone` and `.github` directories for old versions of Go that need updating.

<!-- TODO (ptodev): Update this link after the GH Actions migrations -->
[example-pr]:https://github.com/grafana/alloy/pull/1241

### Step 2: Create a Git tag

After the PR is merged to `main`, a maintainer must push a tag matching the pattern 
`build-image/vX.Y.Z` to the `grafana/alloy` repo. 
For example, to create version `0.1.1` of the build images,
a maintainer would push the tag `build-image/v0.1.1`:

```
git checkout main
git pull
git tag -s build-image/v0.1.1
git push origin build-image/v0.1.1
```

Automation will trigger off of this tag being pushed, 
building and pushing the new build images to Docker Hub.

A follow-up commit to use the newly pushed build images must be made.

<!-- TODO (ptodev): Add a link to an example PR for this follow-up commit -->
