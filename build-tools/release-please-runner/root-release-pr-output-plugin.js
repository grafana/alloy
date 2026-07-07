import { writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { ManifestPlugin } from 'release-please/build/src/plugin.js';
import { writeGithubOutput } from './github-output.js';

export class RootReleasePrOutputPlugin extends ManifestPlugin {
  async run(candidates) {
    if (!process.env.GITHUB_OUTPUT) {
      return candidates;
    }

    const rootCandidate = candidates.find(candidate => candidate.path === '.');
    if (!rootCandidate) {
      return candidates;
    }

    const rootPullRequest = rootCandidate.pullRequest;
    const bodyPath = join(process.env.RUNNER_TEMP || process.cwd(), 'release-please-root-pr-body.md');
    writeFileSync(bodyPath, rootPullRequest.body.toString());
    writeGithubOutput('root_pr_title', rootPullRequest.title.toString());
    writeGithubOutput('root_pr_body_path', bodyPath);
    writeGithubOutput('root_pr_branch', rootPullRequest.headRefName);

    return candidates;
  }
}
