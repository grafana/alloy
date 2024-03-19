# Publish Release

This is how to publish the release in GitHub.

## Before you begin

1. You should see a new draft release created [here](https://github.com/grafana/agent/releases). If not go back to [Tag Release](./4-tag-release.md).

## Steps

1. Edit the release draft by filling in the `Notable Changes` section with all `Breaking Changes` and `Features` from the CHANGELOG.md.

2. Add any additional changes that you think are notable to the list.

3. Add a footer to the `Notable Changes` section:

    `For a full list of changes, please refer to the [CHANGELOG](https://github.com/grafana/agent/blob/RELEASE_VERSION/CHANGELOG.md)!`
    
    Do not substitute the value for `CHANGELOG`.  

4. At the bottom of the release page, perform the following:
    - Tick the check box to "add a discussion" under the category for "announcements".
    - For a Release Candidate, tick the checkbox to "pre-release".
    - For a Stable Release or Patch Release, tick the checkbox to "set as the latest release".

5. Optionally, have other team members review the release draft if you wish
   to feel more comfortable with it.

6. Publish the release!