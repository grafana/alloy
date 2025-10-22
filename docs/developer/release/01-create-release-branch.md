# Create Release Branch

A single release branch is created for every major or minor release. That release
branch is then used for all Release Candidates, the Stable Release, and all
Patch Releases for that version of Alloy.

## Before you begin

1. Determine the [VERSION_PREFIX](concepts/version.md).

## Steps

1. Determine which commit should be used as a base for the release branch.

2. Create and push the release branch from the selected base commit:

    The name of the release branch should be `release/VERSION_PREFIX`
    defined above, such as `release/v0.31`.

        > **NOTE**: Branches are only made for VERSION_PREFIX; do not create branches for the full VERSION such as `release/v0.31-rc.0` or `release/v0.31.0`.

    - If the consensus commit is the latest commit from main you can branch from main.
    - If the consensus commit is not the latest commit from main, branch from that instead.

    > **NOTE**: Don't create any other branches that are prefixed with `release` when creating PRs or
    those branches will collide with our automated release build publish rules.

3. Open a PR against `main` to update the VERSION file at the root of the
   repository to the next minor release planned. For example, if you have just
   created `release/v1.0`, then VERSION should be updated to `v1.1.0`.
