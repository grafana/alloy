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

## Release Candidate Publish
0. [Ensure our OpenTelemetry Collector dependency has been updated](./0-ensure-otel-dep-updated.md)
1. [Create Release Branch](./1-create-release-branch.md)
2. [Cherry Pick Commits](./2-cherry-pick-commits.md)
3. [Update Version in Code](./3-update-version-in-code.md)
4. [Tag Release](./4-tag-release.md)
5. [Publish Release](./6-publish-release.md)
6. [Test Release](./7-test-release.md)
7. [Announce Release](./10-announce-release.md)

## Additional Release Candidate[s] Publish
1. [Cherry Pick Commits](./2-cherry-pick-commits.md)
2. [Update Version in Code](./3-update-version-in-code.md)
3. [Tag Release](./4-tag-release.md)
4. [Publish Release](./6-publish-release.md)
5. [Test Release](./7-test-release.md)
6. [Announce Release](./10-announce-release.md)

## Stable Release Publish
1. [Cherry Pick Commits](./2-cherry-pick-commits.md)
2. [Update Version in Code](./3-update-version-in-code.md)
3. [Tag Release](./4-tag-release.md)
4. [Publish Release](./6-publish-release.md)
5. [Test Release](./7-test-release.md)
6. [Update Helm Charts](./8-update-helm-charts.md)
7. [Update Homebrew](./9-update-homebrew.md)
8. [Announce Release](./10-announce-release.md)
9. [Update OTEL Contrib](./11-update-otel.md)

## Patch Release Publish (latest version)
1. [Cherry Pick Commits](./2-cherry-pick-commits.md)
2. [Update Version in Code](./3-update-version-in-code.md)
3. [Tag Release](./4-tag-release.md)
4. [Publish Release](./6-publish-release.md)
5. [Update Helm Charts](./8-update-helm-charts.md)
6. [Update Homebrew](./9-update-homebrew.md)
7. [Announce Release](./10-announce-release.md)

## Patch Release Publish (older version)
- Not documented yet (but here are some hints)
  - somewhat similar to Patch Release Publish (latest version)
  - find the old release branch
  - cherry-pick commit[s] into it
  - don't update the version in the project on main
  - changes go into the changelog under the patch release version plus stay in unreleased
  - don't publish in github as latest release
  - don't update deployment tools or helm charts
