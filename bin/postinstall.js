#!/usr/bin/env node

import { ensureTerrainBinary } from './terrain-installer.js';

try {
  await ensureTerrainBinary({ quiet: false });
} catch (error) {
  process.stderr.write(
    `[mapterrain] Warning: ${error.message}\n` +
      '[mapterrain] The `terrain` command will try again on first run.\n'
  );
}
