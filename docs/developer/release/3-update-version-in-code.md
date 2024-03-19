# Update Version in Code

The project must be updated to reference the upcoming release tag whenever a new release is being made.

## Before you begin

1. Determine the [VERSION](concepts/version.md).

2. Determine the [VERSION_PREFIX](concepts/version.md)

## Steps

1. Create a branch from `main` for [grafana/agent](https://github.com/grafana/agent).

2. Update the `CHANGELOG.md`:

    1. `CHANGELOG.md` Header
        - First Release Candidate or a Patch Release
            - Add a new header under `Main (unreleased)` for `VERSION (YYYY-MM-DD)`.
        - Additional RCV or SRV
            - Update the header `PREVIOUS_RELEASE_CANDIDATE_VERSION (YYYY-MM-DD)` to `VERSION (YYYY-MM-DD)`. The date may need updating.

    2. Move the unreleased changes we want to add to the release branch from `Main (unreleased)` to `VERSION (YYYY-MM-DD)`.

    3. Update appropriate places in the codebase that have the previous version with the new version determined above.

        First update `tools/gen-versioned-files/agent-version.txt` with the new `VERSION` and run:

        ```
        make generate-versioned-files
        ```

        Next, commit the changes (including those to `tools/gen-versioned-files/agent-version.txt`, as a workflow will use this version to ensure that the templates and generated files are in sync).

        * Do **not** update the `operations/helm` directory. It is updated independently from Agent releases.

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

6. Create a PR to merge to `release/VERSION_PREFIX` (must be merged before continuing).

    - Release Candidate example PR [here](https://github.com/grafana/agent/pull/3066)
    - Stable Release example PR [here](https://github.com/grafana/agent/pull/3123)
    - Patch Release example PR [here](https://github.com/grafana/agent/pull/3193)
        - The `CHANGELOG.md` was updated in cherry-pick commits prior for this example. Make sure it is all set on this PR.
