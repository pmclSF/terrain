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
  checksumFromManifest,
  targetForPlatform,
  verifyChecksumDigest,
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

test('checksumFromManifest matches archive basename', () => {
  const manifest = `
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  terrain_0.3.0_linux_amd64.tar.gz
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  ./nested/terrain_0.3.0_darwin_arm64.tar.gz
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  *terrain_0.3.0_windows_amd64.zip
`;

  assert.equal(
    checksumFromManifest(manifest, 'terrain_0.3.0_linux_amd64.tar.gz'),
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
  );
  assert.equal(
    checksumFromManifest(manifest, 'terrain_0.3.0_darwin_arm64.tar.gz'),
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
  );
  assert.equal(
    checksumFromManifest(manifest, 'terrain_0.3.0_windows_amd64.zip'),
    'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
  );
  assert.equal(
    checksumFromManifest(manifest, 'terrain_0.3.0_linux_arm64.tar.gz'),
    null
  );
});

test('verifyChecksumDigest accepts matching checksum and rejects mismatches', () => {
  const archiveName = 'terrain_0.3.0_linux_amd64.tar.gz';
  const good =
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
  const bad =
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
  const manifest = `${good}  ${archiveName}\n`;

  assert.equal(verifyChecksumDigest(manifest, archiveName, good), good);
  assert.equal(
    verifyChecksumDigest(manifest, archiveName, good.toUpperCase()),
    good
  );
  assert.throws(
    () => verifyChecksumDigest(manifest, archiveName, bad),
    /checksum mismatch/
  );
  assert.throws(
    () =>
      verifyChecksumDigest(manifest, 'terrain_0.3.0_linux_arm64.tar.gz', good),
    /did not contain an entry/
  );
});

test('targetForPlatform matches the published archive matrix', () => {
  assert.deepEqual(targetForPlatform('linux', 'x64'), {
    goos: 'linux',
    goarch: 'amd64',
    archiveExt: 'tar.gz',
    binaryName: 'terrain',
  });
  assert.deepEqual(targetForPlatform('darwin', 'arm64'), {
    goos: 'darwin',
    goarch: 'arm64',
    archiveExt: 'tar.gz',
    binaryName: 'terrain',
  });
  assert.deepEqual(targetForPlatform('win32', 'x64'), {
    goos: 'windows',
    goarch: 'amd64',
    archiveExt: 'zip',
    binaryName: 'terrain.exe',
  });
  assert.throws(
    () => targetForPlatform('win32', 'arm64'),
    /Unsupported prebuilt Terrain target windows\/arm64/
  );
  assert.throws(
    () => targetForPlatform('freebsd', 'x64'),
    /GitHub Releases, Homebrew on macOS\/Linux, or source/
  );
});

// Cleanup: leave the host without a stale marker.
test.after(async () => {
  await clearInstallFailureMarker();
});
