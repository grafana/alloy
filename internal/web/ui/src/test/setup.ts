import '@testing-library/jest-dom/vitest';

import { writeFileSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

import { generateLargeDiscOutput } from './fixtures/generateLargeDiscOutput';

// Generate the JSON fixture file for mock API development server
const __dirname = dirname(fileURLToPath(import.meta.url));
const fixturePath = join(__dirname, 'fixtures', 'large_disc_output.json');
const fixtureData = generateLargeDiscOutput(20000);
writeFileSync(fixturePath, JSON.stringify(fixtureData, null, 2));
