import assert from 'node:assert/strict';
import test from 'node:test';

import { Version } from 'release-please/build/src/version.js';
import { HelmAlloyCommitInjector } from './helm-alloy-commit-injector.js';

const HELM_PATH = 'operations/helm/charts/alloy';

test('adds a Helm fix commit for an Alloy patch candidate', async () => {
  const plugin = new HelmAlloyCommitInjector();
  const strategies = {
    '.': {
      buildReleasePullRequest: async () => ({
        version: Version.parse('1.19.3'),
      }),
    },
    [HELM_PATH]: {},
  };
  const commitsByPath = {
    '.': [{ sha: 'abc123', message: 'feat: Add an Alloy feature' }],
    [HELM_PATH]: [],
  };

  const releasesByPath = {
    '.': { tag: { version: Version.parse('1.19.2') } },
  };

  const result = await plugin.preconfigure(strategies, commitsByPath, releasesByPath);

  assert.equal(result, strategies);
  assert.deepEqual(commitsByPath[HELM_PATH], [
    {
      sha: '',
      message: 'fix(helm): Update to Grafana Alloy v1.19.3',
    },
  ]);
});

test('adds a Helm feature commit for an Alloy minor candidate', async () => {
  const plugin = new HelmAlloyCommitInjector();
  const strategies = {
    '.': {
      buildReleasePullRequest: async () => ({
        version: Version.parse('1.20.0'),
      }),
    },
    [HELM_PATH]: {},
  };
  const commitsByPath = {
    '.': [{ sha: 'abc123', message: 'feat: Add an Alloy feature' }],
    [HELM_PATH]: [],
  };
  const releasesByPath = {
    '.': { tag: { version: Version.parse('1.19.2') } },
  };

  await plugin.preconfigure(strategies, commitsByPath, releasesByPath);

  assert.deepEqual(commitsByPath[HELM_PATH], [
    {
      sha: '',
      message: 'feat(helm): Update to Grafana Alloy v1.20.0',
    },
  ]);
});

test('does not add a Helm commit when Alloy would not version', async () => {
  const plugin = new HelmAlloyCommitInjector();
  const strategies = {
    '.': {
      buildReleasePullRequest: async () => undefined,
    },
    [HELM_PATH]: {},
  };
  const commitsByPath = {
    '.': [],
    [HELM_PATH]: [],
  };

  await plugin.preconfigure(strategies, commitsByPath, {});

  assert.deepEqual(commitsByPath[HELM_PATH], []);
});
