# Shepherding releases

Time to cut a release? Don't worry! It's pretty straightforward. Below are all the situations you
might encounter and how to handle them.

## Cutting a new MINOR release

### Prerequisites

- [OTel dependencies](./updating-otel/README.md) should be updated every ~6 weeks.
- Prometheus dependencies should be updated every ~6 weeks.

### 1. Run the `Create Release Branch` workflow

**_NOTE: Creating a release branch should be considered as "cutting off" the release. Past this
point, only critical fixes should be merged into the branch until the release is final._**

Run the workflow using the GitHub CLI:

```sh
gh workflow run release-create-branch.yml --repo grafana/alloy --field dry_run=false
```

Alternatively, trigger it from the Actions page on github.com by leaving everything as it is except unchecking the `Dry run` box.

This will create a new release branch, a new backport tag, and open a draft release-please PR.

### 2. When ready, cut an RC by running the `Create Release Candidate` workflow

1. Run the workflow using either the GitHub CLI or github.com.
   - **From the GitHub CLI**
      - Run the following, replacing `<VERSION>` with the release branch (e.g. `v1.14`):

        ```sh
        gh workflow run release-create-rc.yml --repo grafana/alloy --ref release/<VERSION> --field dry_run=false
        ```
   - **From github.com**
      - Navigate to the pinned workflow on the Actions page.
      - Select the release branch under `Use workflow from`.
      - Uncheck the `Dry run` box.
   - This will trigger workflows to create a tag for the RC, draft a release on GitHub, build the release artifacts, and attach them to the release.

2. Once everything is attached, add any relevant changelog details to the RC draft release and publish it from either the CLI or github.com. For example:

   ```sh
   gh release edit <VERSION>-rc.0 --draft=false --repo grafana/alloy
   ```

### 3. Validate the RC on internal deployments

1. Deploy the RC to internal clusters following the [Argo Workflows documentation](https://github.com/grafana/alloy-internal/tree/main/Argo-Workflows) in the internal repo.
2. Validate performance metrics are consistent with the prior version.
3. Validate components are healthy.

### 4. (Optional) Add critical fixes to the release

If you find issues during validation, check out the section below on backporting fixes to a release
branch. Once fixes are merged, cut a new RC and repeat step 3.

### 5. When ready, cut the release

1. Move the release-please PR out of draft and review it.
   1. You might realize that some changelog entries don't look the way you want. To address that,
      check out the section below on modifying a PR's changelog entry after it's been merged.
2. Merge the release-please PR.
3. This will trigger workflows to create a draft release on GitHub, build the release artifacts,
   and attach them to the release.
4. Once everything is attached, publish the release.

### 6. Update Helm Chart

1. Create PR against `main` to update the helm chart code as follows:
   1. Update `Chart.yaml` with the new helm version and app version.
   2. Update `CHANGELOG.md` with a new section for the helm version.
   3. Run `make docs rebuild-tests` from the `operations/helm` directory.

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
:alloy: Grafana Alloy <RELEASE_VERSION> is now available! :alloy:
Release: https://github.com/grafana/alloy/releases/tag/<RELEASE_VERSION>
Full changelog: https://github.com/grafana/alloy/blob/<RELEASE_VERSION>/CHANGELOG.md
```

> **Note:** The internal Alloy channel is automatically notified via GitHub Workflow.

## Cutting a new PATCH release

The process for this is exactly the same as a minor release with two notable exceptions:

1. You don't need to create a release branch. Just continue using the appropriate minor release
   branch.
2. You need to ensure that the changes on the release branch are **only resulting in a patch version
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

> **NOTE**: For backport PRs, **do not modify** the PR title, Commit message, or Extended
> description.

## Recreating the release-please PR

If things get stuck and it seems like the solution might just be to regenerate the release-please
PR, follow these steps:

1. Remove the `autorelease: pending` label from the existing PR.
2. Close the PR and delete the head branch.
3. Go go the Actions page, find the release-please workflow, find the most recent entry for the
   target branch (e.g. `release/v.1.12`), and re-run it.
4. (The PR will automatically get recreated.)

## Manually forwardporting a release branch to `main`

The forwardport PRs are what allow the changelog, manifest, and related files to be kept up to date
on the main branch so that subsequent releases have an appropriate starting point when looking for
changes.

When a release-please PR is merged, a workflow automatically forwardports its content by pushing a
branch, opening a draft PR (so the zizmor check runs), waiting for zizmor to pass, then pushing to
`main` and deleting the temp branch. If that workflow fails, use the steps below to forwardport
manually.

1. `git checkout` and `git pull` both the release branch and the main branch
2. Run `git checkout main`
3. Run `git log -1 release/vA.B` and retain the commit hash
4. Add a Bypass to the `Important branches require pull requests (except trusted apps)` Ruleset for
   the `Repository admin` role.
5. Run the following commands:
   ```bash
   git merge --strategy ours origin/release/vN.M --message "chore: Forwardport release A.B.C to main"
   git cherry-pick --no-commit <release_please_commit_hash_from_earlier>
   git commit --amend --no-edit
   git checkout -b tmp/manual-forwardport-for-A.B.C
   git push -u origin tmp/manual-forwardport-for-A.B.C
   gh pr create --draft --base main --title "chore: Forwardport release A.B.C to main"
   ```
6. **DO NOT MERGE THIS PR**. This is only needed to get a passing `zizmor` check for the new commit.
7. Once `zizmor` reports green, run the following commands:
   ```bash
   git checkout main
   git push
   ```
8. The PR will automatically close itself.
9. **Delete the Bypass** from `Important branches require pull requests (except trusted apps)` for
   the `Repository admin` role.
