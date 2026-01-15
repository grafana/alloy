import { writeFileSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

import { largeDiscOutput } from './generateLargeDiscOutput';

const __dirname = dirname(fileURLToPath(import.meta.url));
const jsonPath = join(__dirname, 'large_disc_output.json');

writeFileSync(jsonPath, JSON.stringify(largeDiscOutput, null, 2));

console.log(`Generated fixture at ${jsonPath}`);
