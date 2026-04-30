#!/usr/bin/env node

import { execFileSync, spawn } from 'child_process';
import { createWriteStream, existsSync } from 'fs';
import fs from 'fs/promises';
import https from 'https';
import os from 'os';
import path from 'path';
import { pipeline } from 'stream/promises';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(__dirname, '..');
const packageJson = JSON.parse(
  await fs.readFile(path.join(packageRoot, 'package.json'), 'utf8')
);

const GITHUB_OWNER = 'pmclSF';
const GITHUB_REPO = 'terrain';

function currentTarget() {
  const goosMap = {
    darwin: 'darwin',
    linux: 'linux',
    win32: 'windows',
  };
  const goarchMap = {
    x64: 'amd64',
    arm64: 'arm64',
  };

  const goos = goosMap[process.platform];
  const goarch = goarchMap[process.arch];
  if (!goos || !goarch) {
    throw new Error(
      `Unsupported platform ${process.platform}/${process.arch}. ` +
        'Install Terrain manually from GitHub Releases or via Homebrew.'
    );
  }

  return {
    goos,
    goarch,
    archiveExt: goos === 'windows' ? 'zip' : 'tar.gz',
    binaryName: goos === 'windows' ? 'terrain.exe' : 'terrain',
  };
}

function isDevelopmentCheckout(rootDir = packageRoot) {
  return existsSync(path.join(rootDir, 'cmd', 'terrain'));
}

function installedBinaryPath(rootDir = packageRoot) {
  const target = currentTarget();
  return path.join(
    rootDir,
    'vendor',
    'terrain',
    `${target.goos}-${target.goarch}`,
    target.binaryName
  );
}

function localBuiltBinaryPath(rootDir = packageRoot) {
  const target = currentTarget();
  return path.join(rootDir, target.binaryName);
}

function archiveFileName(version) {
  const target = currentTarget();
  return `terrain_${version}_${target.goos}_${target.goarch}.${target.archiveExt}`;
}

function archiveDownloadUrl(version) {
  const baseUrl =
    process.env.TERRAIN_INSTALLER_BASE_URL ||
    `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download`;
  return `${baseUrl}/v${version}/${archiveFileName(version)}`;
}

function signatureDownloadUrl(version) {
  return `${archiveDownloadUrl(version)}.sig`;
}

function certificateDownloadUrl(version) {
  return `${archiveDownloadUrl(version)}.pem`;
}

function expectedSignerIdentity(version) {
  // The keyless Sigstore signature is anchored to the GitHub Actions workflow
  // that ran goreleaser at release time. The workflow runs on the v<version>
  // tag, so the OIDC subject identity is deterministic.
  return (
    `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}` +
    `/.github/workflows/release.yml@refs/tags/v${version}`
  );
}

const SIGSTORE_OIDC_ISSUER = 'https://token.actions.githubusercontent.com';

function isCosignAvailable() {
  try {
    execFileSync('cosign', ['version'], { stdio: 'pipe' });
    return true;
  } catch {
    return false;
  }
}

// Best-effort signature verification. In 0.1.2 this is warn-only: a missing
// cosign, missing signature artifact, or verification failure logs to stderr
// and does NOT block install. The signing pipeline is still maturing and we
// don't want to break npm installs while it stabilises.
//
// In 0.2 this becomes hard-fail unless TERRAIN_INSTALLER_SKIP_VERIFY=1 is set,
// at which point the warning escalates to an error.
async function verifySignatureBestEffort({
  archivePath,
  version,
  tempDir,
  quiet,
  env,
}) {
  if (env.TERRAIN_INSTALLER_SKIP_VERIFY === '1') {
    log(
      'Skipping signature verification (TERRAIN_INSTALLER_SKIP_VERIFY=1).',
      quiet
    );
    return { verified: false, reason: 'skipped-by-env' };
  }

  if (!isCosignAvailable()) {
    log(
      'cosign not found on PATH; skipping signature verification. ' +
        'Install cosign (https://github.com/sigstore/cosign) for stronger ' +
        'integrity guarantees in future releases.',
      quiet
    );
    return { verified: false, reason: 'cosign-missing' };
  }

  const sigPath = path.join(tempDir, `${path.basename(archivePath)}.sig`);
  const certPath = path.join(tempDir, `${path.basename(archivePath)}.pem`);

  try {
    await downloadFile(signatureDownloadUrl(version), sigPath);
    await downloadFile(certificateDownloadUrl(version), certPath);
  } catch (error) {
    log(
      `Could not fetch signature artifacts (${error.message}); ` +
        'skipping verification.',
      quiet
    );
    return { verified: false, reason: 'sig-download-failed' };
  }

  try {
    execFileSync(
      'cosign',
      [
        'verify-blob',
        '--certificate',
        certPath,
        '--signature',
        sigPath,
        '--certificate-identity',
        expectedSignerIdentity(version),
        '--certificate-oidc-issuer',
        SIGSTORE_OIDC_ISSUER,
        archivePath,
      ],
      { stdio: 'pipe' }
    );
    log(
      `Verified Sigstore signature for ${path.basename(archivePath)}.`,
      quiet
    );
    return { verified: true, reason: 'ok' };
  } catch (error) {
    log(
      `WARNING: cosign verify-blob failed for ${path.basename(archivePath)}. ` +
        'The downloaded archive may be tampered with. Continuing install ' +
        '(verification will become mandatory in 0.2). Error: ' +
        (error.stderr ? error.stderr.toString().trim() : error.message),
      quiet
    );
    return { verified: false, reason: 'verify-failed' };
  }
}

async function ensureDirectory(dir) {
  await fs.mkdir(dir, { recursive: true });
}

async function copyBinary(sourcePath, destinationPath) {
  await ensureDirectory(path.dirname(destinationPath));
  await fs.copyFile(sourcePath, destinationPath);
  if (process.platform !== 'win32') {
    await fs.chmod(destinationPath, 0o755);
  }
}

function log(message, quiet = false) {
  if (!quiet) {
    process.stderr.write(`${message}\n`);
  }
}

async function downloadFile(url, destinationPath) {
  await new Promise((resolve, reject) => {
    const request = https.get(
      url,
      {
        headers: {
          'User-Agent': `${packageJson.name}/${packageJson.version}`,
        },
      },
      async (response) => {
        if (
          response.statusCode &&
          response.statusCode >= 300 &&
          response.statusCode < 400 &&
          response.headers.location
        ) {
          response.resume();
          try {
            await downloadFile(response.headers.location, destinationPath);
            resolve();
          } catch (error) {
            reject(error);
          }
          return;
        }

        if (response.statusCode !== 200) {
          response.resume();
          reject(
            new Error(
              `download failed with HTTP ${response.statusCode} for ${url}`
            )
          );
          return;
        }

        try {
          await pipeline(response, createWriteStream(destinationPath));
          resolve();
        } catch (error) {
          reject(error);
        }
      }
    );

    request.on('error', reject);
  });
}

function extractArchive(archivePath, extractDir) {
  if (archivePath.endsWith('.tar.gz')) {
    execFileSync('tar', ['-xzf', archivePath, '-C', extractDir], {
      stdio: 'pipe',
    });
    return;
  }

  try {
    execFileSync('tar', ['-xf', archivePath, '-C', extractDir], {
      stdio: 'pipe',
    });
  } catch (error) {
    if (process.platform !== 'win32') {
      throw error;
    }
    execFileSync(
      'powershell.exe',
      [
        '-NoLogo',
        '-NoProfile',
        '-Command',
        `Expand-Archive -LiteralPath '${archivePath}' -DestinationPath '${extractDir}' -Force`,
      ],
      { stdio: 'pipe' }
    );
  }
}

async function findBinary(dir, binaryName) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isFile() && entry.name === binaryName) {
      return fullPath;
    }
    if (entry.isDirectory()) {
      const nested = await findBinary(fullPath, binaryName);
      if (nested) {
        return nested;
      }
    }
  }
  return null;
}

export async function ensureTerrainBinary({
  rootDir = packageRoot,
  quiet = false,
  version = packageJson.version,
  env = process.env,
} = {}) {
  const binaryPath = installedBinaryPath(rootDir);
  if (existsSync(binaryPath)) {
    return binaryPath;
  }

  const localOverride = env.TERRAIN_INSTALLER_LOCAL_BINARY;
  if (localOverride && existsSync(localOverride)) {
    log(`Using local Terrain binary override: ${localOverride}`, quiet);
    await copyBinary(localOverride, binaryPath);
    return binaryPath;
  }

  if (isDevelopmentCheckout(rootDir)) {
    const localBinary = localBuiltBinaryPath(rootDir);
    if (existsSync(localBinary)) {
      return localBinary;
    }
    return null;
  }

  if (env.TERRAIN_INSTALLER_SKIP_DOWNLOAD === '1') {
    throw new Error(
      'Terrain binary download skipped because TERRAIN_INSTALLER_SKIP_DOWNLOAD=1.'
    );
  }

  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'terrain-install-'));
  const archivePath = path.join(tempDir, archiveFileName(version));
  const extractDir = path.join(tempDir, 'extract');

  try {
    log(
      `Downloading Terrain ${version} for ${process.platform}/${process.arch}...`,
      quiet
    );
    await downloadFile(archiveDownloadUrl(version), archivePath);
    await verifySignatureBestEffort({
      archivePath,
      version,
      tempDir,
      quiet,
      env,
    });
    await ensureDirectory(extractDir);
    extractArchive(archivePath, extractDir);

    const extractedBinary = await findBinary(
      extractDir,
      currentTarget().binaryName
    );
    if (!extractedBinary) {
      throw new Error(
        `downloaded archive ${path.basename(archivePath)} did not contain ${currentTarget().binaryName}`
      );
    }

    await copyBinary(extractedBinary, binaryPath);
    log(`Installed Terrain binary to ${binaryPath}`, quiet);
    return binaryPath;
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true });
  }
}

export async function runTerrainCli(argv = process.argv.slice(2)) {
  const rootDir = packageRoot;

  if (isDevelopmentCheckout(rootDir)) {
    const localBinary = localBuiltBinaryPath(rootDir);
    if (existsSync(localBinary)) {
      await runBinary(localBinary, argv);
      return;
    }

    await runBinary('go', ['run', './cmd/terrain', ...argv], rootDir);
    return;
  }

  let binaryPath;
  try {
    binaryPath = await ensureTerrainBinary({ rootDir });
  } catch (error) {
    throw new Error(
      `${error.message}\n\n` +
        'Fallback install options:\n' +
        '  brew install pmclSF/terrain/mapterrain\n' +
        '  go install github.com/pmclSF/terrain/cmd/terrain@latest'
    );
  }

  if (!binaryPath) {
    throw new Error(
      'No Terrain binary is available in this checkout yet. ' +
        'Build it with `go build -o terrain ./cmd/terrain` or run the Go CLI directly.'
    );
  }

  await runBinary(binaryPath, argv);
}

async function runBinary(command, args, cwd = undefined) {
  await new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd,
      stdio: 'inherit',
    });

    child.on('error', reject);
    child.on('exit', (code, signal) => {
      if (signal) {
        process.kill(process.pid, signal);
        return;
      }
      process.exitCode = code ?? 1;
      resolve();
    });
  });
}
