#!/usr/bin/env node
//
// Tests the install-failure marker round-trip:
//
//   1. writeInstallFailureMarker captures the error
//   2. clearInstallFailureMarker removes it
//   3. The marker survives a JSON round-trip with all fields populated
//
// Runs via `node --test scripts/test-installer-marker.mjs`. No deps —
// uses the standard library test runner so this is wired into
// `release:verify` without adding to package.json.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { existsSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import {
  writeInstallFailureMarker,
  clearInstallFailureMarker,
} from '../bin/terrain-installer.js';

const markerPath = path.join(os.homedir(), '.terrain', 'install-failure.log');

test('writeInstallFailureMarker captures the error', async () => {
  await clearInstallFailureMarker();
  const original = new Error(
    'cosign is required to verify the Sigstore signature'
  );
  await writeInstallFailureMarker(original);

  assert.ok(existsSync(markerPath), 'marker file should exist after write');

  const body = JSON.parse(await fs.readFile(markerPath, 'utf8'));
  assert.equal(body.message, original.message, 'message preserved');
  assert.ok(body.timestamp, 'timestamp populated');
  assert.match(body.platform, /\//, 'platform string has goos/goarch shape');
  assert.ok(body.version, 'version populated from package.json');
});

test('clearInstallFailureMarker removes the marker', async () => {
  await writeInstallFailureMarker(new Error('temporary'));
  assert.ok(existsSync(markerPath), 'precondition: marker exists');

  await clearInstallFailureMarker();
  assert.ok(!existsSync(markerPath), 'marker should be gone after clear');
});

test('clearInstallFailureMarker is idempotent (no-op when missing)', async () => {
  await clearInstallFailureMarker();
  await clearInstallFailureMarker(); // must not throw
});

// Cleanup: leave the host without a stale marker.
test.after(async () => {
  await clearInstallFailureMarker();
});
