/**
 * Tag-only releases for component packages.
 *
 * release-please's skip-github-release skips both the GitHub Release and
 * force-tag-creation (tags are created inside createRelease). For packages that
 * need a git tag but must not publish a GitHub Release (so they don't steal
 * "latest" from the root Alloy module), we:
 *
 * 1. Clear skipGithubRelease so buildRelease still produces candidates
 * 2. Wrap github.createRelease to create a lightweight tag only
 */

/**
 * Paths that should get a git tag but no GitHub Release.
 * Requires both skipGithubRelease and forceTag on the package config.
 *
 * @param {Record<string, { skipGithubRelease?: boolean, forceTag?: boolean }>} repositoryConfig
 * @returns {Set<string>}
 */
export function getTagOnlyPathsFromConfig(repositoryConfig) {
  const paths = new Set();
  for (const [path, config] of Object.entries(repositoryConfig)) {
    if (config.skipGithubRelease && config.forceTag) {
      paths.add(path);
    }
  }
  return paths;
}

/**
 * Allow release-please to build release candidates for tag-only packages.
 * Must run before createReleases() so strategies are constructed without skip.
 *
 * @param {{ repositoryConfig: Record<string, { skipGithubRelease?: boolean }> }} manifest
 * @param {Set<string>} tagOnlyPaths
 */
export function prepareTagOnlyPackages(manifest, tagOnlyPaths) {
  for (const path of tagOnlyPaths) {
    manifest.repositoryConfig[path].skipGithubRelease = false;
  }
}

/**
 * Wrap github.createRelease so tag-only package paths create a lightweight
 * refs/tags/* ref and skip repos.createRelease.
 *
 * @param {import('release-please').GitHub} github
 * @param {Set<string>} tagOnlyPaths
 */
export function installTagOnlyReleaseHandler(github, tagOnlyPaths) {
  if (tagOnlyPaths.size === 0) {
    return;
  }

  const original = github.createRelease.bind(github);

  github.createRelease = async (release, options = {}) => {
    const path = release.path ?? '.';
    if (!tagOnlyPaths.has(path)) {
      return original(release, options);
    }

    const tagName = release.tag.toString();
    const sha = release.sha;
    console.log(`Tag-only ${path}: creating lightweight tag ${tagName} -> ${sha}`);

    try {
      await github.octokit.git.createRef({
        owner: github.repository.owner,
        repo: github.repository.repo,
        ref: `refs/tags/${tagName}`,
        sha,
      });
    } catch (err) {
      // Match release-please force-tag-creation: ignore already-exists.
      if (err.status === 422) {
        console.log(`Tag ${tagName} already exists, skipping tag creation`);
      } else {
        throw err;
      }
    }

    const { owner, repo } = github.repository;
    return {
      id: 0,
      name: release.name,
      tagName,
      sha,
      notes: release.notes,
      // Tag page works without a GitHub Release object.
      url: `https://github.com/${owner}/${repo}/releases/tag/${tagName}`,
      draft: false,
      uploadUrl: '',
    };
  };
}
