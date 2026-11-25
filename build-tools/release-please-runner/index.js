#!/usr/bin/env node

/**
 * Based on https://github.com/googleapis/release-please-action/blob/main/src/index.ts
 * Adapted for CLI usage with custom versioning strategy.
 */

import { GitHub, Manifest, VERSION } from 'release-please';
import { registerVersioningStrategy } from 'release-please/build/src/factories/versioning-strategy-factory.js';
import { MinorBreakingVersioningStrategy } from './minor-breaking-versioning.js';
import { HelmAlloyCommitInjector } from './plugins/helm-alloy-commit-injector.js';
import { outputReleasePullRequests } from './release-pr-output.js';
import {
  installTagOnlyReleaseHandler,
  prepareTagOnlyPackages,
  getTagOnlyPathsFromConfig,
} from './tag-only-releases.js';
import { parseArgs } from 'node:util';
import { fileURLToPath } from 'node:url';

// Register the custom versioning strategy
registerVersioningStrategy('minor-breaking', (options) => new MinorBreakingVersioningStrategy(options));

const DEFAULT_CONFIG_FILE = 'release-please-config.json';
const DEFAULT_MANIFEST_FILE = '.release-please-manifest.json';

export const usage = `Usage: node index.js [options]

Options:
  --include <path>  Keep only this package path in the manifest. Repeatable.
                    Packages that are not listed are dropped. If omitted, all
                    packages are kept.
  -h, --help        Show this help.
`;

/**
 * Returns a parser for the runner CLI. Unknown flags are rejected.
 *
 * Options:
 *   --include <path>  Keep only this package path. Repeatable.
 *   -h, --help        Show usage.
 */
export function createArgParser() {
  return (argv = process.argv.slice(2)) => {
    const { values } = parseArgs({
      args: argv,
      options: {
        include: {
          type: 'string',
          multiple: true,
          default: [],
        },
        help: {
          type: 'boolean',
          short: 'h',
          default: false,
        },
      },
    });
    return { include: values.include, help: values.help };
  };
}

function parseInputs(argv = process.argv.slice(2)) {
  const parse = createArgParser();
  const cliArgs = parse(argv);
  if (cliArgs.help) {
    return { help: true, include: cliArgs.include };
  }

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
    include: cliArgs.include,
  };
}

function keepPaths(entries, paths) {
  return Object.fromEntries(Object.entries(entries).filter(([path]) => paths.has(path)));
}

export function keepIncludedPackages(manifest, paths) {
  const included = new Set(paths);
  manifest.repositoryConfig = keepPaths(manifest.repositoryConfig, included);
  manifest.releasedVersions = keepPaths(manifest.releasedVersions, included);
}

async function loadManifest(github, inputs) {
  console.log('Loading manifest from config file');
  const manifest = await Manifest.fromManifest(
    github,
    inputs.targetBranch || github.repository.defaultBranch,
    inputs.configFile,
    inputs.manifestFile,
  );

  if (inputs.include.length > 0) {
    console.log(`Keeping packages: ${inputs.include.join(', ')}`);
    keepIncludedPackages(manifest, inputs.include);
  }

  manifest.plugins.unshift(new HelmAlloyCommitInjector());

  return manifest;
}

async function main() {
  const inputs = parseInputs();
  if (inputs.help) {
    console.log(usage);
    return;
  }

  console.log(`Running release-please version: ${VERSION}`);
  const github = await getGitHubInstance(inputs);

  if (!inputs.skipGitHubRelease) {
    const manifest = await loadManifest(github, inputs);
    const tagOnlyPaths = getTagOnlyPathsFromConfig(manifest.repositoryConfig);
    console.log(
      `Tag-only packages (git tag, no GitHub Release): ${
        tagOnlyPaths.size > 0 ? [...tagOnlyPaths].join(', ') : '(none)'
      }`
    );

    prepareTagOnlyPackages(manifest, tagOnlyPaths);
    installTagOnlyReleaseHandler(github, tagOnlyPaths);

    console.log('Creating releases');
    outputReleases(await manifest.createReleases());
  }

  if (!inputs.skipGitHubPullRequest) {
    const manifest = await loadManifest(github, inputs);
    console.log('Creating pull requests');

    const prs = await manifest.createPullRequests();
    // Emit after createPullRequests so outputs can reflect a post-Merge combined PR, if used
    outputReleasePullRequests(prs);
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

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch(err => {
    console.error(`release-please failed: ${err.message}`);
    process.exit(1);
  });
}
