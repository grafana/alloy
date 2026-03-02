#!/usr/bin/env node

/**
 * Based on https://github.com/googleapis/release-please-action/blob/main/src/index.ts
 * Adapted for CLI usage with custom versioning strategy.
 */

import { GitHub, Manifest, VERSION } from 'release-please';
import { Octokit } from '@octokit/rest';
import { registerVersioningStrategy } from 'release-please/build/src/factories/versioning-strategy-factory.js';
import { MinorBreakingVersioningStrategy } from './minor-breaking-versioning.js';

// Register the custom versioning strategy
registerVersioningStrategy('minor-breaking', (options) => new MinorBreakingVersioningStrategy(options));

const DEFAULT_CONFIG_FILE = 'release-please-config.json';
const DEFAULT_MANIFEST_FILE = '.release-please-manifest.json';

function parseInputs() {
  const token = process.env.GITHUB_TOKEN;
  if (!token) {
    throw new Error('GITHUB_TOKEN environment variable is required');
  }

  const repoUrl = process.env.REPO_URL || process.env.GITHUB_REPOSITORY || '';
  if (!repoUrl) {
    throw new Error('REPO_URL or GITHUB_REPOSITORY environment variable is required');
  }

  return {
    token,
    repoUrl,
    targetBranch: process.env.TARGET_BRANCH || undefined,
    configFile: process.env.CONFIG_FILE || DEFAULT_CONFIG_FILE,
    manifestFile: process.env.MANIFEST_FILE || DEFAULT_MANIFEST_FILE,
    skipGitHubRelease: process.env.SKIP_GITHUB_RELEASE === 'true',
    skipGitHubPullRequest: process.env.SKIP_GITHUB_PULL_REQUEST === 'true',
    pullRequestTitle: process.env.PULL_REQUEST_TITLE || '',
    pullRequestHeader: process.env.PULL_REQUEST_HEADER || '',
  };
}

function loadManifest(github, inputs) {
  console.log('Loading manifest from config file');
  return Manifest.fromManifest(
    github,
    inputs.targetBranch || github.repository.defaultBranch,
    inputs.configFile,
    inputs.manifestFile
  );
}

async function main() {
  console.log(`Running release-please version: ${VERSION}`);
  const inputs = parseInputs();
  const github = await getGitHubInstance(inputs);

  if (!inputs.skipGitHubRelease) {
    const manifest = await loadManifest(github, inputs);
    console.log('Creating releases');
    outputReleases(await manifest.createReleases());
  }

  if (!inputs.skipGitHubPullRequest) {
    const manifest = await loadManifest(github, inputs);
    console.log('Creating pull requests');

    const prs = await manifest.createPullRequests();
    outputPullRequests(prs);

    await applyPullRequestCustomizations(inputs, prs);
  }
}

/**
 * Update PR title/body after release-please creates them.
 *
 * @param {GitHub} github - The GitHub instance
 * @param {Object} inputs - The input parameters
 * @param {PullRequest[]} prs - The pull requests to update
 */
async function applyPullRequestCustomizations(inputs, prs) {
  const definedPrs = prs.filter(pr => pr !== undefined);
  if (definedPrs.length === 0) {
    return;
  }

  if (!inputs.pullRequestTitle && !inputs.pullRequestHeader) {
    return;
  }

  const [owner, repo] = inputs.repoUrl.split('/');
  const octokit = new Octokit({ auth: inputs.token });

  for (const pr of definedPrs) {
    const updates = {};
    if (inputs.pullRequestTitle) {
      updates.title = inputs.pullRequestTitle;
    }

    if (inputs.pullRequestHeader) {
      updates.body = `${inputs.pullRequestHeader}\n\n${pr.body}`;
    }

    console.log(`Customizing PR #${pr.number} title/body`);
    await octokit.pulls.update({
      owner,
      repo,
      pull_number: pr.number,
      ...updates,
    });
  }
}

function getGitHubInstance(inputs) {
  const [owner, repo] = inputs.repoUrl.split('/');
  return GitHub.create({
    owner,
    repo,
    token: inputs.token,
    defaultBranch: inputs.targetBranch,
  });
}

function outputReleases(releases) {
  releases = releases.filter(release => release !== undefined);
  const pathsReleased = [];
  console.log(`releases_created=${releases.length > 0}`);
  for (const release of releases) {
    const path = release.path || '.';
    pathsReleased.push(path);
    console.log(`Created release: ${release.tagName}`);
    for (const [rawKey, value] of Object.entries(release)) {
      let key = rawKey;
      if (key === 'tagName') key = 'tag_name';
      if (key === 'uploadUrl') key = 'upload_url';
      if (key === 'notes') key = 'body';
      if (key === 'url') key = 'html_url';
      console.log(`  ${key}=${value}`);
    }
  }
  console.log(`paths_released=${JSON.stringify(pathsReleased)}`);
}

function outputPullRequests(prs) {
  prs = prs.filter(pr => pr !== undefined);
  console.log(`prs_created=${prs.length > 0}`);
  if (prs.length) {
    for (const pr of prs) {
      console.log(`Created/updated PR #${pr.number}: ${pr.title}`);
    }
  }
}

main().catch(err => {
  console.error(`release-please failed: ${err.message}`);
  process.exit(1);
});
