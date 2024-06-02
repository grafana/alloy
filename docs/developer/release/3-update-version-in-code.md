# Update Version in Code

The project must be updated to reference the upcoming release tag whenever a new release is being made.

## Before you begin

1. Determine the [VERSION](concepts/version.md).

2. Determine the [VERSION_PREFIX](concepts/version.md)

## Steps

1. Create a branch from `main` for [grafana/alloy](https://github.com/grafana/alloy).

2. Update the `CHANGELOG.md`:

    1. `CHANGELOG.md` Header
        - First Release Candidate or a Patch Release
            - Add a new header under `Main (unreleased)` for `VERSION`.
        - Additional RCV or SRV
            - Update the header `PREVIOUS_RELEASE_CANDIDATE_VERSION` to `VERSION`. The date may need updating.

    2. Move the unreleased changes we want to add to the release branch from `Main (unreleased)` to `VERSION`.

3. Create a PR to merge to main (must be merged before continuing).

    - Release Candidate example PR [here](https://github.com/grafana/agent/pull/3065)
    - Stable Release example PR [here](https://github.com/grafana/agent/pull/3119)
    - Patch Release example PR [here](https://github.com/grafana/agent/pull/3191)

4. If one doesn't exist yet, create a branch called `release/VERSION_PREFIX` for [grafana/alloy](https://github.com/grafana/alloy).

5. Cherry pick the commit on main from the merged PR in Step 3 from main into the branch from Step 4:

    ```
    git cherry-pick -x COMMIT_SHA
    ```

    Delete the `Main (unreleased)` header and anything underneath it as part of the cherry-pick. Alternatively, do it after the cherry-pick is completed.

6. **If you are creating a patch release,** ensure that the file called `VERSION` in your branch matches the version being published, without any release candidate or build information:

   > **NOTE**: Only perform this step for patch releases, and make sure that
   > the change is not pushed to the main branch.

   After updating `VERSION`, run:

   ```bash
   make generate-versioned-files
   ```

   Next, commit the changes (including those to `VERSION`, as a workflow will use this version to ensure that the templates and generated files are in sync).


6. Create a PR to merge to `release/VERSION_PREFIX` (must be merged before continuing).

    - Release Candidate example PR [here](https://github.com/grafana/agent/pull/3066)
    - Stable Release example PR [here](https://github.com/grafana/agent/pull/3123)
    - Patch Release example PR [here](https://github.com/grafana/agent/pull/3193)
        - The `CHANGELOG.md` was updated in cherry-pick commits prior for this example. Make sure it is all set on this PR.
