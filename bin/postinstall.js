#!/usr/bin/env node

import {
  ensureTerrainBinary,
  writeInstallFailureMarker,
  clearInstallFailureMarker,
} from './terrain-installer.js';

// We intentionally don't fail npm install when the binary fetch fails —
// CI pipelines that run `npm install` as part of a larger flow can
// recover from a transient download issue, and forcing every cosign-
// missing host to fail the install would be more disruptive than the
// failure mode itself. But a silent warning is also wrong: a missing
// binary should not be discovered five minutes later when the user
// runs `terrain analyze` and gets a confusing retry.
//
// Compromise: write a marker file describing the failure. The CLI
// trampoline reads it on first run and prints a clear, framed error
// pointing at the remediation (install cosign, or set the documented
// opt-out env var) instead of attempting a silent retry.
try {
  await ensureTerrainBinary({ quiet: false });
  // Clean up any stale marker from a previous failed install.
  await clearInstallFailureMarker();
} catch (error) {
  await writeInstallFailureMarker(error);
  process.stderr.write(
    '\n' +
      '!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n' +
      '! mapterrain: binary install FAILED                              !\n' +
      '!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n' +
      '\n' +
      `${error.message}\n` +
      '\n' +
      'npm install reports success, but the `terrain` binary is NOT\n' +
      'installed. Running `terrain` will fail with the same error\n' +
      'until the underlying issue is resolved.\n' +
      '\n' +
      'Marker written to ~/.terrain/install-failure.log\n' +
      '\n'
  );
}
