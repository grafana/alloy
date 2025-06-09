# Update Homebrew

After a stable or patch release is created, a bot will automatically create a PR in the [homebrew-grafana][] repository.
The PR will bump the version of Alloy in Alloy's Brew formula.

## Steps

1. Navigate to the [homebrew-grafana][] repository.

2. Find the PR which bumps the Alloy formula to the release that was just published. It will look like [this one][example-pr].

3. Merge the PR.

[homebrew-grafana]: https://github.com/grafana/homebrew-grafana
[example-pr]: https://github.com/grafana/homebrew-grafana/pull/89