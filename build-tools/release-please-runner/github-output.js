import { appendFileSync } from 'node:fs';

export function writeGithubOutput(name, value) {
  if (!process.env.GITHUB_OUTPUT) {
    return;
  }

  const outputValue = String(value);
  if (!outputValue.includes('\n')) {
    appendFileSync(process.env.GITHUB_OUTPUT, `${name}=${outputValue}\n`);
    return;
  }

  let delimiter = `EOF_${name.toUpperCase()}`;
  while (outputValue.includes(delimiter)) {
    delimiter += '_';
  }
  appendFileSync(process.env.GITHUB_OUTPUT, `${name}<<${delimiter}\n${outputValue}\n${delimiter}\n`);
}
