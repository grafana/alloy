# Update the "main" and "release/" branches

You need to update the CHANGELOG and the [VERSION file][VERSION-file].
You may also need to cherry pick commits on the `release/` branch.

## Before you begin

1. Determine the [VERSION](concepts/version.md).

2. Determine the [VERSION_PREFIX](concepts/version.md)

## 1. Update the "release/VERSION_PREFIX" branch

Example PRs:

* [Release Candidate](https://github.com/grafana/alloy/pull/1410).
  Here the PR is done on the main branch, before creating the release branch.
  You can do this to save time by not having to update the release branch separately.
* [Additional Release Candidate](https://github.com/grafana/alloy/pull/1701)
* [Stable Release](https://github.com/grafana/alloy/pull/1747)
  There is no need to update the VERSION file in this PR.
  The VERSION file is already pointing to the version being released ever since the `release/` branch was created.
* [Patch Release](https://github.com/grafana/alloy/pull/1767)

### 1.1. Add the new version to the CHANGELOG

For a First Release Candidate (`rc.0`), add a new header below the `Main (unreleased)` for `VERSION`. For example:
```
Main (unreleased)
-----------------

v1.12.0-rc.0
-----------------

(Release notes that were previously under Main (unreleased))
```

For an Additional Release Candidate or SRV, increment the number in the release candidate header to match your new
`VERSION`. For example, `v1.12.0-rc.0` becomes `v1.12.0-rc.1`.

For a patch release, add a new header for `VERSION`.

### 1.2. Cherry pick commits

If you need certain changes on the release branch but they're not yet there, cherry-pick them onto the release branch.
In the CHANGELOG, make sure they are listed under the header for the new VERSION and not under `Main (unreleased)`.

### 1.3. Update the VERSION file

The [VERSION file][VERSION-file] is used by the CI to ensure that templates and generated files are in sync.

The VERSION file on the `release/` branch should point to the stable (or patch) version you're about to release.

The contents of the VERSION file should not contain `rc` information.
Therefore, there is no need to update the VERSION file for additional release candidates (e.g. `rc.1`, `rc.2`).

For example:
* If you are going to release `v1.2.0-rc.0`, then the VERSION file should contain `v1.2.0`.
* If you are going to release `v1.5.1`, then the VERSION file should contain `v1.5.1`.

After updating the VERSION file, run:

```bash
make generate-versioned-files
```

## 2. Update the "main" branch

Examples:

* Release Candidate example PR [here](https://github.com/grafana/alloy/pull/1410)
* Stable Release example PR [here](https://github.com/grafana/alloy/pull/1419)
* Patch Release example PR [here](https://github.com/grafana/alloy/pull/1769)

### 2.1. Add the new version to the CHANGELOG

For a First Release Candidate or a Patch Release, add a new header under `Main (unreleased)` for `VERSION`.

For an Additional Release Candidate or SRV, update the header `PREVIOUS_RELEASE_CANDIDATE_VERSION` to `VERSION`.

### 2.2. Update the VERSION file

The [VERSION file][VERSION-file] on the "main" branch should point to the next minor version.
For example:
* If you are going to release `v1.2.0-rc.0`, then the VERSION file should contain `v1.3.0`.
* If you are going to release `v1.5.1`, then the VERSION file should contain `v1.6.0`.

The reasoning behind this is that any builds of the main branch should contain the next minor version they are meant to go to.
If the latest release branch that was cut is `release/1.3`, then `main` is preparing for `1.4.0`.
Any builds of the main branch will therefore be labeled `devel-1.4.0`.

After updating the VERSION file, run:

```bash
make generate-versioned-files
```

[VERSION-file]: https://github.com/grafana/alloy/blob/main/VERSION
