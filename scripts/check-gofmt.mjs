import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

const root = 'backend';
const files = [];

function walk(path) {
  for (const name of readdirSync(path)) {
    if (name === 'vendor') continue;
    const fullPath = join(path, name);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) walk(fullPath);
    else if (name.endsWith('.go')) files.push(fullPath);
  }
}

walk(root);
let failed = false;
for (const file of files) {
  const result = spawnSync('gofmt', ['-d', file], { encoding: 'utf8' });
  if (result.error || (result.status !== 0 && result.status !== 1)) {
    process.stderr.write(result.stderr || `gofmt failed for ${file}\n`);
    failed = true;
    continue;
  }
  if (result.status === 1 && result.stdout.trim()) {
    if (process.argv.includes('--write')) {
      const formatted = spawnSync('gofmt', [file], { encoding: 'utf8' });
      if (formatted.error || formatted.status !== 0) failed = true;
      else writeFileSync(file, formatted.stdout || readFileSync(file, 'utf8'));
    } else {
      process.stderr.write(result.stdout);
      failed = true;
    }
  }
}

if (failed) process.exitCode = 1;
