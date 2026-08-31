import { parseConventionalCommits } from 'release-please/build/src/commit.js';

const ROOT_PATH = '.';
const HELM_PATH = 'operations/helm/charts/alloy';

/**
 * Injects a synthetic conventional commit into the Helm package when Alloy
 * would get a new version, so the chart bumps with it: `fix` for a patch,
 * `feat` otherwise. No-ops if either strategy is missing.
 */
export class HelmAlloyCommitInjector {
  async preconfigure(strategies, commitsByPath, releasesByPath) {
    if (!strategies[ROOT_PATH] || !strategies[HELM_PATH]) {
      // Do nothing unless both Alloy and Helm release strategies are present
      return strategies;
    }

    const alloyRelease = await strategies[ROOT_PATH].buildReleasePullRequest(
      parseConventionalCommits(commitsByPath[ROOT_PATH]),
      releasesByPath[ROOT_PATH],
    );
    if (!alloyRelease?.version) {
      return strategies;
    }

    const previousVersion = releasesByPath[ROOT_PATH]?.tag.version;
    const commitType =
      previousVersion?.major === alloyRelease.version.major &&
      previousVersion?.minor === alloyRelease.version.minor
        ? 'fix'
        : 'feat';

    commitsByPath[HELM_PATH].push({
      sha: '',
      message: `${commitType}(helm): Update to Grafana Alloy v${alloyRelease.version}`,
    });
    return strategies;
  }

  processCommits(commits) {
    return commits;
  }

  async run(candidates) {
    return candidates;
  }
}
