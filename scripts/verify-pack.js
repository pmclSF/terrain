#!/usr/bin/env node

/**
 * Verify that npm pack produces a valid, installable CLI wrapper package.
 * Run via: npm test or npm run release:verify.
 */

import { execFileSync } from 'child_process';
import fs from 'fs/promises';
import path from 'path';
import os from 'os';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

let tmpDir;
let tarball;
let npmCacheDir;
let builtBinaryDir;

try {
  const packageJson = JSON.parse(
    await fs.readFile(path.join(rootDir, 'package.json'), 'utf8')
  );
  const packageName = packageJson.name;
  const packageVersion = packageJson.version;

  npmCacheDir = await fs.mkdtemp(path.join(os.tmpdir(), 'terrain-npm-cache-'));
  builtBinaryDir = await fs.mkdtemp(path.join(os.tmpdir(), 'terrain-binary-'));
  const localBinary = path.join(
    builtBinaryDir,
    process.platform === 'win32' ? 'terrain.exe' : 'terrain'
  );

  execFileSync(
    'go',
    [
      'build',
      '-ldflags',
      `-X main.version=${packageVersion} -X main.commit=verify-pack -X main.date=1970-01-01T00:00:00Z`,
      '-o',
      localBinary,
      './cmd/terrain',
    ],
    { cwd: rootDir, encoding: 'utf8' }
  );

  const npmEnv = {
    ...process.env,
    NPM_CONFIG_CACHE: npmCacheDir,
    npm_config_cache: npmCacheDir,
    TERRAIN_INSTALLER_LOCAL_BINARY: localBinary,
  };

  console.log('Packing tarball...');
  const packOutput = execFileSync(
    'npm',
    ['pack', '--pack-destination', os.tmpdir()],
    { cwd: rootDir, encoding: 'utf8', env: npmEnv }
  ).trim();

  const tarballName = packOutput.split('\n').pop().trim();
  tarball = path.join(os.tmpdir(), tarballName);
  console.log(`  Created: ${tarballName}`);

  // List contents for review
  console.log('\nTarball contents:');
  const listing = execFileSync('tar', ['tzf', tarball], { encoding: 'utf8' });
  const files = listing.trim().split('\n');
  console.log(`  ${files.length} files`);

  // Check for unexpected files — anything outside the intended npm surface.
  const unexpected = files.filter(
    (f) =>
      f.includes('node_modules/') ||
      f.includes('.env') ||
      f.includes('.github/') ||
      f.includes('/test/') ||
      f.includes('/tests/') ||
      f.includes('/internal/') ||
      f.includes('/cmd/') ||
      f.includes('/benchmarks/') ||
      f.includes('/scripts/') ||
      f.includes('/fixtures/') ||
      f.includes('/docs/') ||
      f.includes('/extension/') ||
      f.includes('.goreleaser') ||
      f.includes('go.mod') ||
      f.includes('go.sum') ||
      f.includes('Makefile') ||
      f.includes('CLAUDE.md') ||
      f.includes('DESIGN.md')
  );
  if (unexpected.length > 0) {
    console.error('\nUnexpected files in tarball:');
    unexpected.forEach((f) => console.error(`  ${f}`));
    process.exit(1);
  }

  // Install in temp dir and verify the packaged CLI wrapper.
  console.log('\nInstalling in temp directory...');
  tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'terrain-verify-'));
  execFileSync('npm', ['init', '-y'], {
    cwd: tmpDir,
    encoding: 'utf8',
    env: npmEnv,
  });
  execFileSync('npm', ['install', tarball], {
    cwd: tmpDir,
    encoding: 'utf8',
    env: npmEnv,
  });

  // CLI smoke tests
  console.log('CLI smoke tests...');
  const terrainBin = path.join(tmpDir, 'node_modules', '.bin', 'terrain');
  const packageBin = path.join(tmpDir, 'node_modules', '.bin', packageName);

  const pkgVersion = JSON.parse(
    await fs.readFile(
      path.join(tmpDir, 'node_modules', packageName, 'package.json'),
      'utf8'
    )
  ).version;
  const terrainVersion = JSON.parse(
    execFileSync(terrainBin, ['version', '--json'], {
      encoding: 'utf8',
    })
  );
  if (terrainVersion.version !== pkgVersion) {
    console.error(
      `  terrain version mismatch: got "${terrainVersion.version}", expected "${pkgVersion}"`
    );
    process.exit(1);
  }
  console.log(`  terrain version --json: ok (${terrainVersion.version})`);

  const listConversions = JSON.parse(
    execFileSync(terrainBin, ['list-conversions', '--json'], {
      encoding: 'utf8',
    })
  );
  const directionCount = Array.isArray(listConversions.categories)
    ? listConversions.categories.reduce(
        (total, category) =>
          total +
          (Array.isArray(category.directions) ? category.directions.length : 0),
        0
      )
    : 0;
  if (directionCount === 0) {
    console.error('  terrain list-conversions returned no directions');
    process.exit(1);
  }
  console.log(
    `  terrain list-conversions --json: ok (${directionCount} directions)`
  );

  const packageVersionJson = JSON.parse(
    execFileSync(packageBin, ['version', '--json'], {
      encoding: 'utf8',
    })
  );
  if (packageVersionJson.version !== pkgVersion) {
    console.error(
      `  ${packageName} version mismatch: got "${packageVersionJson.version}", expected "${pkgVersion}"`
    );
    process.exit(1);
  }
  console.log(`  ${packageName} alias: ok`);

  const analyzeRepoDir = path.join(tmpDir, 'analyze-fixture');
  await fs.mkdir(analyzeRepoDir, { recursive: true });
  await fs.writeFile(
    path.join(analyzeRepoDir, 'sample.test.js'),
    `describe('Smoke', () => {
  it('works', () => {
    expect(true).toBe(true);
  });
});
`
  );
  const analyzeOut = execFileSync(
    terrainBin,
    ['analyze', '--root', analyzeRepoDir],
    { encoding: 'utf8' }
  );
  if (!analyzeOut.includes('Terrain')) {
    console.error('  terrain analyze output did not look valid');
    process.exit(1);
  }
  console.log('  terrain analyze smoke: ok');

  // Conversion smoke test: jest -> vitest on a tiny inline fixture.
  console.log('\nConversion smoke test...');
  const fixtureDir = path.join(tmpDir, 'fixture');
  const convertOutDir = path.join(tmpDir, 'converted');
  const migrateOutDir = path.join(tmpDir, 'migrated');
  await fs.mkdir(fixtureDir, { recursive: true });
  await fs.writeFile(
    path.join(fixtureDir, 'smoke.test.js'),
    `describe('Smoke', () => {
  it('should pass', () => {
    expect(true).toBe(true);
  });
});
`
  );

  execFileSync(
    terrainBin,
    [
      'convert',
      fixtureDir,
      '--from',
      'jest',
      '--to',
      'vitest',
      '-o',
      convertOutDir,
    ],
    { encoding: 'utf8' }
  );

  const convertedFiles = await fs.readdir(convertOutDir);
  if (convertedFiles.length === 0) {
    console.error('  No output files produced');
    process.exit(1);
  }
  const convertedContent = await fs.readFile(
    path.join(convertOutDir, convertedFiles[0]),
    'utf8'
  );
  if (!convertedContent.includes('describe') || convertedContent.length < 10) {
    console.error('  Converted output looks invalid');
    process.exit(1);
  }
  console.log(
    `  jest→vitest: ok (${convertedFiles.length} file, ${convertedContent.length} bytes)`
  );

  console.log('\nMigration workflow smoke test...');
  execFileSync(
    terrainBin,
    [
      'migrate',
      fixtureDir,
      '--from',
      'jest',
      '--to',
      'vitest',
      '-o',
      migrateOutDir,
    ],
    { encoding: 'utf8' }
  );
  const migrationStatus = JSON.parse(
    execFileSync(terrainBin, ['status', '--dir', fixtureDir, '--json'], {
      encoding: 'utf8',
    })
  );
  if (!migrationStatus.exists || migrationStatus.status.converted === 0) {
    console.error('  migration status did not report converted files');
    process.exit(1);
  }
  console.log(
    `  terrain migrate/status: ok (${migrationStatus.status.converted} converted)`
  );

  console.log('\nRelease verification passed.');
} catch (error) {
  console.error('Release verification failed:', error.message);
  process.exit(1);
} finally {
  if (tmpDir) await fs.rm(tmpDir, { recursive: true, force: true });
  if (tarball) await fs.rm(tarball, { force: true });
  if (npmCacheDir) await fs.rm(npmCacheDir, { recursive: true, force: true });
  if (builtBinaryDir) {
    await fs.rm(builtBinaryDir, { recursive: true, force: true });
  }
}
