#!/usr/bin/env node

/**
 * Based on https://github.com/googleapis/release-please-action/blob/main/src/index.ts
 * Adapted for CLI usage with custom versioning strategy.
 */

import { GitHub, Manifest, VERSION } from 'release-please';
import { registerVersioningStrategy } from 'release-please/build/src/factories/versioning-strategy-factory.js';
import { registerPlugin } from 'release-please/build/src/factories/plugin-factory.js';
import { MinorBreakingVersioningStrategy } from './minor-breaking-versioning.js';
import { ReleasePrOutputPlugin } from './release-pr-output-plugin.js';
import { fileURLToPath } from 'node:url';

// Register the custom versioning strategy
registerVersioningStrategy('minor-breaking', (options) => new MinorBreakingVersioningStrategy(options));
registerPlugin('release-pr-output', (options) => {
  return new ReleasePrOutputPlugin(options.github, options.targetBranch, options.repositoryConfig);
});

const DEFAULT_CONFIG_FILE = 'release-please-config.json';
const DEFAULT_MANIFEST_FILE = '.release-please-manifest.json';
const ROOT_PACKAGE_PATH = '.';

/**
 * Parse CLI flags from argv (process.argv.slice(2) by default).
 * Unknown flags are rejected.
 */
export function parseArgs(argv = process.argv.slice(2)) {
  const args = {
    rootOnly: false,
  };

  for (const arg of argv) {
    switch (arg) {
      case '--root-only':
        args.rootOnly = true;
        break;
      default:
        throw new Error(`Unknown argument: ${arg}`);
    }
  }

  return args;
}

function parseInputs(argv = process.argv.slice(2)) {
  const token = process.env.GITHUB_TOKEN;
  if (!token) {
    throw new Error('GITHUB_TOKEN environment variable is required');
  }

  const repoUrl = process.env.REPO_URL || process.env.GITHUB_REPOSITORY || '';
  if (!repoUrl) {
    throw new Error('REPO_URL or GITHUB_REPOSITORY environment variable is required');
  }

  const cliArgs = parseArgs(argv);

  return {
    token,
    repoUrl,
    targetBranch: process.env.TARGET_BRANCH || undefined,
    configFile: process.env.CONFIG_FILE || DEFAULT_CONFIG_FILE,
    manifestFile: process.env.MANIFEST_FILE || DEFAULT_MANIFEST_FILE,
    skipGitHubRelease: process.env.SKIP_GITHUB_RELEASE === 'true',
    skipGitHubPullRequest: process.env.SKIP_GITHUB_PULL_REQUEST === 'true',
    rootOnly: cliArgs.rootOnly,
  };
}

function loadManifest(github, inputs) {
  const onlyPath = inputs.rootOnly ? ROOT_PACKAGE_PATH : undefined;
  if (onlyPath) {
    console.log(`Loading manifest from config file (root-only: path=${onlyPath})`);
  } else {
    console.log('Loading manifest from config file');
  }
  return Manifest.fromManifest(
    github,
    inputs.targetBranch || github.repository.defaultBranch,
    inputs.configFile,
    inputs.manifestFile,
    {},
    onlyPath
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
    logPullRequests(prs);
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

function logPullRequests(prs) {
  prs = prs.filter(pr => pr !== undefined);
  console.log(`prs_created=${prs.length > 0}`);
  if (prs.length) {
    for (const pr of prs) {
      console.log(`Created/updated PR #${pr.number}: ${pr.title}`);
    }
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch(err => {
    console.error(`release-please failed: ${err.message}`);
    process.exit(1);
  });
}
