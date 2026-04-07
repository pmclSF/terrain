#!/usr/bin/env node

import { runTerrainCli } from './terrain-installer.js';

try {
  await runTerrainCli();
} catch (error) {
  process.stderr.write(`${error.message}\n`);
  process.exit(1);
}
