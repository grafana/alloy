# Shepherding releases

Time to cut a release? Don't worry! It's pretty straightforward. Below are all the situations you
might encounter and how to handle them.

## Cutting a new MINOR release

### Prerequisites

- [OTel dependencies](./updating-otel/README.md) should be updated every ~6 weeks.
- Prometheus dependencies should be updated every ~6 weeks.

### 1. Run the `Create Release Branch` pinned workflow on the Actions page

**_NOTE: Creating a release branch should be considered as "cutting off" the release. Past this
point, only critical fixes should be merged into the branch until the release is final._**

1. Leave everything as it is except uncheck the `Dry run` box.
2. (This will create a new release branch, a new backport tag, and open a draft release-please PR.)

### 2. When ready, cut an RC by running the `Create Release Candidate` pinned workflow on the Actions page

1.  For `Use workflow from`, select the release branch in question.
2.  Make sure to uncheck the `Dry run` box.
3.  (This will create a draft release on GitHub.)
4.  Add any relevant changelog details to the RC draft release and publish it.
5.  (This will trigger CI to build/publish release artifacts and attach them to the release.)

### 3. (Optional) Add critical fixes to the release

1. If you need to add critical fixes to the release branch after testing an RC, check out the
   section below on backporting fixes to a release branch.

### 4. When ready, move the release-please PR out of draft, review it, and merge it in

1.  You might realize that some changelog entries don't look the way you want. To address that,
    check out the section below on modifying a PR's changelog entry after it's been merged.
1.  (This will trigger CI to build/publish release artifacts and attach them to the release.)

### 5. Validate the release on internal deployments

1.  Validate performance metrics are consistent with the prior version.
2.  Validate components are healthy.

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
   feat: this is the overridden semantic commit title
   END_COMMIT_OVERRIDE
   ```

If you need to mark something as a **breaking change**, use the following:

```
BEGIN_COMMIT_OVERRIDE
feat!: this is the overridden semantic commit title

BREAKING-CHANGE: This is where you write a detailed description about the breaking change. You can use markdown if needed.
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

If this job should happen to fail when a release-please PR gets merged into a release branch, here
are the steps you can take to do it manually:

1. `git checkout` and `git pull` both the release branch and the main branch
2. Run `git checkout main`
3. Run `git log -1 release/vA.B` and retain the commit hash
4. Add a Bypass to the `Important branches require pull requests (except trusted apps)` Ruleset for
   the `Repository admin` role.
5. Run the following commands:
   ```bash
   git merge --strategy ours origin/release/vN.M --message "chore: forwardport release A.B.C to main"
   git cherry-pick --no-commit <release_please_commit_hash_from_earlier>
   git commit --amend --no-edit
   git push
   ```
6. **Delete the Bypass** from `Important branches require pull requests (except trusted apps)` for
   the `Repository admin` role.
