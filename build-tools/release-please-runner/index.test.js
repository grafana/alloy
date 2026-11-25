import assert from 'node:assert/strict';
import test from 'node:test';

import { createArgParser, keepIncludedPackages, usage } from './index.js';

const parseArgs = createArgParser();

test('parseArgs collects repeated --include paths', () => {
  assert.deepEqual(parseArgs(['--include', '.', '--include', 'syntax']), {
    include: ['.', 'syntax'],
    help: false,
  });
});

test('parseArgs collects --include= paths', () => {
  assert.deepEqual(parseArgs(['--include=.', '--include=syntax']), {
    include: ['.', 'syntax'],
    help: false,
  });
});

test('parseArgs defaults include to an empty list', () => {
  assert.deepEqual(parseArgs([]), { include: [], help: false });
});

test('parseArgs accepts --help', () => {
  assert.equal(parseArgs(['--help']).help, true);
  assert.equal(parseArgs(['-h']).help, true);
});

test('usage documents --include as an allowlist', () => {
  assert.match(usage, /--include <path>/);
  assert.match(usage, /Keep only this package path in the manifest/);
  assert.match(usage, /Packages that are not listed are dropped/);
});

test('parseArgs rejects --include without a path', () => {
  assert.throws(() => parseArgs(['--include']), /Option '--include <value>' argument missing/);
  assert.throws(() => parseArgs(['--include', '--include', '.']), /Option '--include' argument is ambiguous/);
});

test('parseArgs rejects unknown arguments', () => {
  assert.throws(() => parseArgs(['--root-only']), /Unknown option '--root-only'/);
});

test('keepIncludedPackages keeps only the requested paths', () => {
  const manifest = {
    repositoryConfig: {
      '.': {},
      syntax: {},
      'operations/helm/charts/alloy': {},
    },
    releasedVersions: {
      '.': '1.20.0',
      syntax: '0.2.0',
      'operations/helm/charts/alloy': '1.13.0',
    },
  };

  keepIncludedPackages(manifest, ['.', 'operations/helm/charts/alloy']);

  assert.deepEqual(Object.keys(manifest.repositoryConfig), [
    '.',
    'operations/helm/charts/alloy',
  ]);
  assert.deepEqual(Object.keys(manifest.releasedVersions), [
    '.',
    'operations/helm/charts/alloy',
  ]);
});
