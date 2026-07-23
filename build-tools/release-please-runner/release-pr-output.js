import { writeGithubOutput } from './github-output.js';

/**
 * Write workflow outputs after createPullRequests() returns. Must run after
 * release-please's Merge plugin so the matrix sees the single combined PR
 * (title, body, branch), not per-package pre-merge candidates.
 *
 * @param {Array<{ headBranchName?: string, title?: string, body?: string, number?: number }|undefined|null>} prs
 */
export function outputReleasePullRequests(prs) {
  prs = (prs || []).filter(pr => pr != null);
  const pullRequests = prs.map(pr => ({
    branch: pr.headBranchName,
    title: pr.title,
    body: pr.body,
  }));

  console.log(`prs_created=${pullRequests.length > 0}`);
  for (const pr of prs) {
    console.log(`Created/updated PR #${pr.number}: ${pr.title}`);
  }

  writeGithubOutput('release_prs_created', pullRequests.length > 0);
  writeGithubOutput('release_prs', JSON.stringify(pullRequests));
}
