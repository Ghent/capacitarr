#!/usr/bin/env node
/**
 * Generate frontend/app/types/generated/openapi.ts from the published OpenAPI spec.
 * Do not hand-edit the output. Run via `make api:generate` or `pnpm api:generate`.
 */
import { spawnSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const spec = resolve(frontendRoot, '../docs/reference/api/openapi.yaml');
const out = resolve(frontendRoot, 'app/types/generated/openapi.ts');

mkdirSync(dirname(out), { recursive: true });

const result = spawnSync(
  'pnpm',
  ['exec', 'openapi-typescript', spec, '-o', out],
  { cwd: frontendRoot, stdio: 'inherit' },
);

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
