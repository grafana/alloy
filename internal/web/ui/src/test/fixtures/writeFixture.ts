import { mkdirSync, writeFileSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

import { heavyDiscOutput, lightDiscOutput } from './generateLargeDiscOutput';

const __dirname = dirname(fileURLToPath(import.meta.url));
const generatedDir = join(__dirname, '..', 'generated_fixtures');

// Ensure the generated_fixtures directory exists
mkdirSync(generatedDir, { recursive: true });

// Write heavy fixture (large - requires download button)
const heavyPath = join(generatedDir, 'discovery.kubernetes.heavy.json');
writeFileSync(heavyPath, JSON.stringify(heavyDiscOutput, null, 2));
console.log(`Generated heavy fixture at ${heavyPath}`);

// Write light fixture (small - renders inline)
const lightPath = join(generatedDir, 'discovery.kubernetes.light.json');
writeFileSync(lightPath, JSON.stringify(lightDiscOutput, null, 2));
console.log(`Generated light fixture at ${lightPath}`);
