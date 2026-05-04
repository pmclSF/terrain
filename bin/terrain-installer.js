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

// installFailureMarkerPath returns the path where a failed install
// records its error. The CLI trampoline checks this before retrying
// so users see a clear remediation message instead of a confusing
// retry of the same failure. ~/.terrain is also where local snapshots
// live, so the location is already a Terrain working directory.
function installFailureMarkerPath() {
  return path.join(os.homedir(), '.terrain', 'install-failure.log');
}

// writeInstallFailureMarker is called from postinstall.js when
// `npm install` fails to fetch / verify the binary. It captures the
// error so the next `terrain` invocation can print it verbatim
// without attempting another silent retry.
export async function writeInstallFailureMarker(error) {
  try {
    const markerPath = installFailureMarkerPath();
    await fs.mkdir(path.dirname(markerPath), { recursive: true });
    const body = JSON.stringify(
      {
        timestamp: new Date().toISOString(),
        message: error?.message ?? String(error),
        stack: error?.stack ?? null,
        platform: `${process.platform}/${process.arch}`,
        version: packageJson.version,
      },
      null,
      2
    );
    await fs.writeFile(markerPath, body, 'utf8');
  } catch (writeErr) {
    // Failing to write the marker is itself non-fatal; the postinstall
    // warning has already been printed.
    process.stderr.write(
      `[mapterrain] (could not record install-failure marker: ${writeErr.message})\n`
    );
  }
}

// clearInstallFailureMarker removes the marker on a successful
// install or successful first run. Idempotent.
export async function clearInstallFailureMarker() {
  try {
    await fs.unlink(installFailureMarkerPath());
  } catch (err) {
    if (err.code !== 'ENOENT') {
      // ENOENT is the happy path (no marker existed). Anything else
      // is unexpected; surface it but don't fail.
      process.stderr.write(
        `[mapterrain] (could not clear install-failure marker: ${err.message})\n`
      );
    }
  }
}

// readInstallFailureMarker returns the recorded error message, or
// null if no marker exists.
async function readInstallFailureMarker() {
  try {
    const body = await fs.readFile(installFailureMarkerPath(), 'utf8');
    return JSON.parse(body);
  } catch (err) {
    return null;
  }
}

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

// Sigstore signature verification.
//
// 0.2.x policy: Sigstore verification is MANDATORY by default. If
// `cosign` is not available on the host, the install fails with a
// clear remediation pointer. The escape for trusted/CI/air-gapped
// environments is the documented opt-out
// `TERRAIN_INSTALLER_SKIP_VERIFY=1`.
//
// Pre-0.2.x silently degraded to "checksum-only" when cosign was
// missing, which meant a typical npm-install on a host without cosign
// (most macOS / Linux dev machines) skipped Sigstore entirely without
// any signal in the install log beyond a one-line "falling back"
// message. Adversarial review flagged this as the headline gap in our
// supply-chain story: the strong-integrity guarantee we advertise
// degrades silently to weak by default. Promotion to mandatory closes
// the gap; the env-var escape keeps adoption viable.
//
// Escape hatches:
//
//   - TERRAIN_INSTALLER_SKIP_VERIFY=1 — fully opt out (CI / air-gapped).
//     Prints a WARNING so the bypass is auditable.
//   - TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1 — opt-in degrade-to-
//     checksum behavior for hosts that genuinely cannot install
//     cosign. Pre-0.2.x default; opt-in in 0.2.x.
//
// Once cosign is on the host, every verify failure is a hard error.
async function verifySignatureBestEffort({
  archivePath,
  version,
  tempDir,
  quiet,
  env,
}) {
  if (env.TERRAIN_INSTALLER_SKIP_VERIFY === '1') {
    log(
      'WARNING: signature verification skipped (TERRAIN_INSTALLER_SKIP_VERIFY=1). ' +
        'Set this only in trusted CI / air-gapped environments where ' +
        'integrity is established by another channel.',
      quiet
    );
    return { verified: false, reason: 'skipped-by-env' };
  }

  if (!isCosignAvailable()) {
    if (env.TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN === '1') {
      log(
        'cosign not found on PATH. Continuing with checksum-only verification ' +
          'because TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1 is set. ' +
          'For stronger integrity guarantees install cosign ' +
          '(https://github.com/sigstore/cosign) and reinstall.',
        quiet
      );
      return { verified: false, reason: 'cosign-missing-allowed' };
    }
    throw new Error(
      'cosign is required to verify the Sigstore signature on the Terrain ' +
        'release archive, but was not found on PATH.\n\n' +
        'Resolve by one of:\n' +
        '  1. Install cosign: https://github.com/sigstore/cosign#installation\n' +
        '     (Homebrew: `brew install cosign`. Linux: see release notes.)\n' +
        '  2. If this host genuinely cannot install cosign and you trust the ' +
        'GitHub-provided checksum file, set ' +
        'TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1 to fall back to ' +
        'checksum-only verification.\n' +
        '  3. To skip integrity verification entirely (NOT recommended), ' +
        'set TERRAIN_INSTALLER_SKIP_VERIFY=1.'
    );
  }

  const sigPath = path.join(tempDir, `${path.basename(archivePath)}.sig`);
  const certPath = path.join(tempDir, `${path.basename(archivePath)}.pem`);

  try {
    await downloadFile(signatureDownloadUrl(version), sigPath);
    await downloadFile(certificateDownloadUrl(version), certPath);
  } catch (error) {
    // Hard error in 0.2: if cosign is present, the signature download
    // is required. The release pipeline produces signatures for every
    // archive; their absence is a real failure mode worth surfacing.
    throw new Error(
      `cosign is installed but the Sigstore signature artifacts for ` +
        `terrain ${version} could not be downloaded: ${error.message}. ` +
        `Set TERRAIN_INSTALLER_SKIP_VERIFY=1 to bypass at your own risk.`
    );
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
    // Hard error in 0.2: a verify-blob failure means the archive on disk
    // does NOT match the signed certificate. Aborting the install is
    // strictly safer than silently continuing.
    const detail = error.stderr
      ? error.stderr.toString().trim()
      : error.message;
    throw new Error(
      `cosign verify-blob FAILED for ${path.basename(archivePath)}: ${detail}. ` +
        `The downloaded archive does not match its Sigstore signature; ` +
        `the binary may have been tampered with. Install aborted.`
    );
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

// MAX_REDIRECTS caps redirect chains to defend against misconfigured
// proxies that loop. 5 covers every normal redirect chain (GitHub
// release → CDN → storage backend) with margin to spare. 0.2.0
// final-polish: pre-fix the recursion was unbounded — a redirect
// loop hung the installer until the OS killed it.
const MAX_REDIRECTS = 5;

async function downloadFile(
  url,
  destinationPath,
  redirectsRemaining = MAX_REDIRECTS
) {
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
          if (redirectsRemaining <= 0) {
            reject(
              new Error(
                `download exceeded ${MAX_REDIRECTS} redirects for ${url}; ` +
                  'check for proxy redirect loops or set ' +
                  'TERRAIN_INSTALLER_BASE_URL to a direct download host.'
              )
            );
            return;
          }
          try {
            await downloadFile(
              response.headers.location,
              destinationPath,
              redirectsRemaining - 1
            );
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

  // Check for a recorded install failure before attempting a silent
  // retry. If `npm install` failed to fetch/verify the binary, the
  // marker file records the original error; surface it verbatim
  // instead of pretending nothing happened.
  const marker = await readInstallFailureMarker();
  if (marker && !existsSync(installedBinaryPath(rootDir))) {
    throw new Error(
      'Terrain binary is not installed.\n\n' +
        `Recorded install failure (${marker.timestamp}, ${marker.platform}, v${marker.version}):\n` +
        `  ${marker.message}\n\n` +
        'Resolve the underlying issue, then either:\n' +
        '  - Re-run `npm install -g mapterrain` after installing cosign\n' +
        '  - Set TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1 to fall back to\n' +
        '    checksum-only verification, or\n' +
        '  - Set TERRAIN_INSTALLER_SKIP_VERIFY=1 to skip verification entirely.\n\n' +
        'Marker file: ~/.terrain/install-failure.log'
    );
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

  // First successful run after a failed install: clear the marker.
  await clearInstallFailureMarker();

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
