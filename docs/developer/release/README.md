# Releasing

This document describes the process of creating a release for the
`grafana/alloy` repo. A release includes release assets for everything inside
the repository.

The processes described here are for v1.0 and above.

# Release Cycle

A typical release cycle is to have a Release Candidate published for at least 48
hours followed by a Stable Release. 0 or more Patch Releases may occur between the Stable Release
and the creation of the next Release Candidate.

# Workflows

Once a release is scheduled, a release shepherd is determined. This person will be
responsible for ownership of the following workflows:

## First release candidate (`rc.0`)
1. [Ensure our OpenTelemetry Collector dependency has been updated](./0-ensure-otel-dep-updated.md)
2. [Create Release Branch](./1-create-release-branch.md)
3. [Update the "main" and "release/" branches](./3-update-version-in-code.md)
4. [Tag Release](./4-tag-release.md)
5. [Publish Release](./6-publish-release.md)
6. [Announce Release](./10-announce-release.md)

## Additional release candidate (`rc.1`, `rc.2`...)
1. [Update the "main" and "release/" branches](./3-update-version-in-code.md)
2. [Tag Release](./4-tag-release.md)
3. [Publish Release](./6-publish-release.md)
4. [Announce Release](./10-announce-release.md)

## Stable Release Publish (`1.2.0`, `1.6.0`...)
1. [Update the "main" and "release/" branches](./3-update-version-in-code.md)
2. [Tag Release](./4-tag-release.md)
3. [Publish Release](./6-publish-release.md)
4. [Update Helm Charts](./8-update-helm-charts.md)
5. [Update Homebrew](./9-update-homebrew.md)
6. [Announce Release](./10-announce-release.md)
7. [Update OTEL Contrib](./11-update-otel.md)

## Patch Release Publish - latest version (`1.15.1`, `1.15.2`...)
1. [Update the "main" and "release/" branches](./3-update-version-in-code.md)
2. [Tag Release](./4-tag-release.md)
3. [Publish Release](./6-publish-release.md)
4. [Update Helm Charts](./8-update-helm-charts.md)
5. [Update Homebrew](./9-update-homebrew.md)
6. [Announce Release](./10-announce-release.md)

## Patch Release Publish - older version (`1.0.1`, `1.0.2`...)
- Not documented yet (but here are some hints)
  - somewhat similar to Patch Release Publish (latest version)
  - find the old release branch
  - cherry-pick commit[s] into it
  - don't update the version in the project on main
  - changes go into the changelog under the patch release version plus stay in unreleased
  - don't publish in github as latest release
  - don't update deployment tools or helm charts
