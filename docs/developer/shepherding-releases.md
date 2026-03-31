# Shepherding releases

Time to cut a release? Don't worry! It's pretty straightforward. Below are all the situations you
might encounter and how to handle them.

## Cutting a new MINOR release

### Things to know

1. Minor releases are cut from the `main` branch.
2. Tagging a minor release creates an associated release branch (e.g. release/v1.14).
3. Patch releases are cut from the corresponding release branch.
4. Creating an RC against `main` puts the repo into `Code Freeze` until the release-please PR is
   merged.
   - PRs with conventional commit types that don't affect the binary (e.g. `docs`, `test`, `ci`,
     `style`, `proposal`) are auto-exempted from the freeze.
   - PRs labeled `freeze-exempt` are allowed through the code freeze guard.

### Prerequisites

1. One "key dependency" is updated each release (e.g. Prometheus, OTel). Ensure the current update
   is complete before starting.
   - See also: [Updating OTel](./updating-otel/README.md)
2. Any outstanding CVEs and Renovate security PRs should be addressed/merged before starting.

### 1. Mark the release-please PR on `main` as "Ready for review"

While doing this, review the draft release-please PR and make sure the version updates and changelog
entries look correct.

You might realize that some changelog entries don't look the way you want. To address that, check
out the section below on modifying a PR's changelog entry after it's been merged.

### 2. When ready, cut an RC by running the `Create Release Candidate` workflow

> **NOTE:** This will cause the repo to enter a **Code Freeze** 🥶 until the release-please PR is
> merged. PRs that don't affect the binary (`docs`, `test`, `ci`, `style`, `proposal`) are
> auto-exempted. Other PRs can be labeled `freeze-exempt` to bypass the guard.

1. Run the workflow using either the GitHub CLI or github.com.
   - **From the GitHub CLI**
     - Run the following:
       ```sh
       gh workflow run release-create-rc.yml --repo grafana/alloy --ref main --field dry_run=false
       ```
   - **From github.com**
     - Navigate to the pinned workflow on the Actions page.
     - Select `main` under `Use workflow from`.
     - Uncheck the `Dry run` box.
   - This will trigger workflows to create a tag for the RC, draft a release on GitHub, build the
     release artifacts, and attach them to the release.
   - The first RC automatically creates the `backport/vX.Y` label, so PRs can be labeled for
     backporting during the RC validation phase.
2. Once everything is attached, add any relevant changelog details to the RC draft release and
   publish it from either the CLI or github.com. For example:
   ```sh
   gh release edit <VERSION>-rc.0 --draft=false --repo grafana/alloy
   ```

### 3. Validate the RC on internal deployments

1. Deploy the RC to internal clusters following the
   [Argo Workflows documentation](https://github.com/grafana/alloy-internal/tree/main/Argo-Workflows)
   in the internal repo.
2. Validate performance metrics are consistent with the prior version.
3. Validate components are healthy.

### 4. (Optional) Add critical fixes to the release

If you find issues during validation, merge only critical fixes into `main`. Once fixes are merged,
cut a new RC and repeat step 3.

> **NOTE:** Once an RC has been created, `main` is effectively frozen until the final release. Only
> critical fixes approved by the release manager should be merged. PRs that don't affect the binary
> (e.g. `docs`, `test`, `ci`, `style`) are auto-exempt; other PRs can be labeled `freeze-exempt` to
> bypass the guard.

### 5. When ready, cut the release

1. Merge the release-please PR into `main`.
2. This will trigger workflows to create a draft release on GitHub, build the release artifacts, and
   attach them to the release.
3. Once everything is attached, publish the release either from the CLI or from the Releases page on
   github.com.
4. This will automatically create the corresponding `release/vX.Y` branch (and `backport/vX.Y` label
   if it wasn't already created during the RC phase).

### 6. Update Helm Chart

1. Create a PR against `main` to update the helm chart code.
2. Update `Chart.yaml` with the new helm version and app version.
3. Update `CHANGELOG.md` with a new section for the helm version.
4. Run `make docs rebuild-tests` from the `operations/helm` directory.

### 7. Update Homebrew

**_NOTE: Publishing a release should automatically create a PR against the Homebrew repo to bump the
version. If it doesn't you'll need to do this manually._**

1. Navigate to the [homebrew-grafana](https://github.com/grafana/homebrew-grafana) repository.
2. Find the PR which bumps the Alloy formula to the release that was just published. It will look
   like [this one](https://github.com/grafana/homebrew-grafana/pull/89).
3. If needed, update the contents of the PR with any additional changes needed to support the new
   Alloy version. This might mean updating the Go version, changing build tags, default config file
   contents, etc.
4. Merge the PR.

### 8. Announce

You did it! Now it's time to celebrate by announcing the Alloy release in the following Slack
channels:

- #alloy (internal slack)
- #alloy (community slack)

**Message format:**

```
:alloy: :alloy: :alloy:
*Grafana Alloy <RELEASE_VERSION> is now available!*

*Release:* https://github.com/grafana/alloy/releases/tag/<RELEASE_VERSION>
*Full changelog:* https://github.com/grafana/alloy/blob/<RELEASE_VERSION>/CHANGELOG.md
:alloy: :alloy: :alloy:
```

> **Note:** The internal Alloy channel is automatically notified via GitHub Workflow.

## Cutting a new PATCH release

The process for this is exactly the same as a minor release with two notable exceptions:

1. Backport your changes to the release branch (which is automatically created after the
   corresponding minor is tagged).
2. Look for the release-please PR for the release branch in question.
3. You need to ensure that the changes on the release branch are **only resulting in a patch version
   bump**. If they're not, follow the steps below for modifying PR changelog entries, or if you
   truly goofed and backported a feature instead of a fix, revert it and update changelog entries as
   necessary.

## Modifying a PR's CHANGELOG entry post-merge

By default, the semantics of each commit message (derived from PR titles) become the basis for
changelog entries. If you need to change one after the PR has already been merged, do the following:

1. Navigate to the PR in question
2. Edit the PR's description and append a block such as the following to the bottom of it:
   ```
   BEGIN_COMMIT_OVERRIDE
   feat: this is the overridden conventional commit-style title (#pr_number_goes_here)
   END_COMMIT_OVERRIDE
   ```
3. Re-run the latest release-please workflow run for the branch the PR targets. This will trigger
   the changelog to update.

If you need to mark something as a **breaking change**, use the following:

```
BEGIN_COMMIT_OVERRIDE
feat!: this is the overridden conventional commit-style title (#pr_number_goes_here)

BREAKING-CHANGE: This is where you write a detailed description about the breaking change. You can
use markdown if needed.
END_COMMIT_OVERRIDE
```

## Backporting a fix to a release branch

This is super simple! Once you have a PR with a fix on the `main` branch (doesn't matter if it's
open or merged), do the following:

1. Add a backport label to it (e.g. `backport/v1.12`).
2. Merge the PR.

The order of the above steps does not matter.

Once the PR is merged AND has the label, a backport PR will automatically be created for the PR
against the appropriate release branch. Merge it in and you're done!

If there's no other pending content on the release branch, you'll see a new release-please PR get
created for the next release.

If the automatic cherry-pick fails, the workflow comments on the original PR with manual backport
instructions.

> **NOTE**: For backport PRs, **do not modify** the PR title, Commit message, or Extended
> description.

## Recreating the release-please PR

If things get stuck and it seems like the solution might just be to regenerate the release-please
PR, follow these steps:

1. Remove the `autorelease: pending` label from the existing PR.
2. Close the PR and delete the head branch.
3. Go to the Actions page, find the `release-please` workflow, find the most recent entry for the
   target branch (for example, `main` or `release/v1.12`), and re-run it.
4. (The PR will automatically get recreated.)
