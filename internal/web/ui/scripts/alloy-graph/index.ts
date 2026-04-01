#!/usr/bin/env node
/**
 * alloy-graph — render an Alloy configuration as a component graph SVG.
 *
 * Usage:
 *   npm run graph -- <config.alloy> [-o output.svg]
 *
 * The graph layout uses the same layoutGraph() function as the browser UI,
 * so node sizing and Dagre positioning are identical.
 */

import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

import { layoutGraph } from '../../src/features/graph/layoutGraph.ts';
import { parseConfig } from './parseConfig.ts';
import { renderSVG } from './renderSVG.ts';

function usage(): never {
  console.error(
    [
      'alloy-graph: render an Alloy config as a component graph SVG',
      '',
      'usage: npm run graph -- <config.alloy> [-o output.svg]',
      '',
      'options:',
      '  -o <path>   Write SVG to <path> (default: write to stdout)',
    ].join('\n')
  );
  process.exit(1);
}

function parseArgs(argv: string[]): { configFile: string; outputFile: string | null } {
  let configFile: string | null = null;
  let outputFile: string | null = null;

  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '-o') {
      outputFile = argv[++i] ?? usage();
    } else if (argv[i].startsWith('-')) {
      console.error(`alloy-graph: unknown flag: ${argv[i]}`);
      usage();
    } else {
      if (configFile !== null) usage();
      configFile = argv[i];
    }
  }

  if (configFile === null) usage();
  return { configFile, outputFile };
}

const { configFile, outputFile } = parseArgs(process.argv.slice(2));

let source: string;
try {
  source = fs.readFileSync(path.resolve(configFile), 'utf8');
} catch (err) {
  console.error(`alloy-graph: cannot read ${configFile}: ${(err as Error).message}`);
  process.exit(1);
}

const components = parseConfig(source);
const layout = layoutGraph(components);
const svg = renderSVG(layout);

if (outputFile) {
  try {
    fs.writeFileSync(path.resolve(outputFile), svg, 'utf8');
  } catch (err) {
    console.error(`alloy-graph: cannot write ${outputFile}: ${(err as Error).message}`);
    process.exit(1);
  }
} else {
  process.stdout.write(svg);
}
