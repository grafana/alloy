import { ManifestPlugin } from 'release-please/build/src/plugin.js';
import { writeGithubOutput } from './github-output.js';

export class ReleasePrOutputPlugin extends ManifestPlugin {
  async run(candidates) {
    if (!process.env.GITHUB_OUTPUT) {
      return candidates;
    }

    const pullRequests = candidates
      .filter(candidate => candidate.pullRequest)
      .map(candidate => {
        const pullRequest = candidate.pullRequest;
        return {
          path: candidate.path || '.',
          branch: pullRequest.headRefName,
          title: pullRequest.title.toString(),
          body: pullRequest.body.toString(),
        };
      });

    writeGithubOutput('release_prs_created', pullRequests.length > 0);
    writeGithubOutput('release_prs', JSON.stringify(pullRequests));

    return candidates;
  }
}
