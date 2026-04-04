# Releasing Terrain

## Release Flow

```
git tag vX.Y.Z && git push origin vX.Y.Z
         │
         ▼
  release.yml (trigger: tag push v*)
    ├── verify job:
    │     ├── actions/setup-go + actions/setup-node
    │     ├── Assert tag matches package.json version
    │     └── make release-verify
    │           ├── make go-release-verify
    │           │     ├── go vet ./cmd/... ./internal/...
    │           │     ├── go test ./cmd/... ./internal/...
    │           │     ├── go build ./cmd/terrain
    │           │     └── go test ./cmd/terrain/ -run TestSnapshot -count=1 -v
    │           ├── make js-release-verify
    │           │     ├── npm ci
    │           │     ├── format:check
    │           │     ├── lint
    │           │     ├── test (all suites)
    │           │     └── verify-pack.js
    │           │           ├── npm pack → install in temp dir
    │           │           ├── Verify JS exports (VERSION, convertFile, …)
    │           │           ├── CLI smoke (`terrain-convert`, `terrain`, `mapterrain`)
    │           │           └── Conversion smoke (jest→vitest)
    │           └── make extension-verify
    │                 ├── npm --prefix extension/vscode ci
    │                 ├── npm --prefix extension/vscode run compile
    │                 └── npm --prefix extension/vscode test
    ├── go-release job (needs: verify):
    │     ├── goreleaser release --clean
    │     ├── Create GitHub Release with binaries + SBOMs + checksums
    │     └── Update Homebrew tap formula in pmclSF/homebrew-terrain
    └── npm-release job (needs: verify + go-release):
          ├── npm ci
          └── npm publish --provenance (NPM_TOKEN secret)
```

A single workflow (`release.yml`) handles the full pipeline: verify → GitHub/Homebrew release → npm release. The npm package publishes only after the GitHub release artifacts exist, because the `mapterrain` npm package installs the Go CLI from those tagged assets.

### Go Binary Release

The `go-release` job uses [GoReleaser](https://goreleaser.com/) to build
multi-platform binaries (Linux/macOS/Windows × amd64/arm64) and attach them
to the GitHub Release. Configuration lives in `.goreleaser.yaml`.

Binaries are stamped with version, commit, and build date via ldflags:
```
-X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}
```

Users can install via:
```bash
go install github.com/pmclSF/terrain/cmd/terrain@latest
```
Users can also download pre-built binaries from the GitHub Releases page, install with Homebrew, or use npm:

```bash
brew install pmclSF/terrain/mapterrain
npm install -g mapterrain
```

The verify job includes a **tag/version guard** that aborts if the git tag
version does not match `package.json` version — preventing accidental publishes
of mismatched versions.

`publish.yml` (renamed "Verify Release") is a safety net that triggers on
`release: created` events. It runs `make release-verify` but does NOT
publish — this catches issues if a release is created manually outside the
tag-push flow.

### Required Secrets

| Secret | Where | Purpose |
|--------|-------|---------|
| `NPM_TOKEN` | GitHub repo → Settings → Secrets | npm automation token with publish access |
| `HOMEBREW_TAP_GITHUB_TOKEN` | GitHub repo → Settings → Secrets | push generated formula updates to `pmclSF/homebrew-terrain` |

### Permissions

| Workflow | Permission | Why |
|----------|-----------|-----|
| `release.yml` | `contents: write` | Create GitHub Release |
| `release.yml` | `id-token: write` | npm provenance attestation (`npm publish --provenance`) |
| `publish.yml` | `contents: read` | Read repo (verify only, no publish) |

## Dry-Run Checklist

Follow these steps before cutting any release.

### Prerequisites

- [ ] You are on `main` with a clean working tree (`git status` shows nothing)
- [ ] CI is green on the latest commit (check GitHub Actions)
- [ ] `CHANGELOG.md` is up to date for the new version
- [ ] `package.json` version matches the tag you are about to create

### 1. Run the full verification locally

```bash
make release-verify
```

This runs the Go release gate, the npm package verification, and the VS Code
extension compile path in one contract.

### 2. Inspect tarball contents

```bash
npm pack --dry-run
```

Confirm only expected files are included:
- `bin/terrain-cli.js`
- `bin/terrain-installer.js`
- `bin/postinstall.js`
- `bin/terrain.js`
- `src/**/*.js`
- `src/types/*.d.ts`
- `README.md`
- `SECURITY.md`
- `LICENSE`
- `package.json`

No test files, no `.github/`, no `node_modules/`, no `.env`.

### 3. Tag and push

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

### 4. Verify after push

- [ ] GitHub Actions: `release.yml` completed successfully
- [ ] GitHub Releases: new release exists with binaries, checksums, and SBOMs
- [ ] Homebrew: `brew install pmclSF/terrain/mapterrain` succeeds
- [ ] npm: `npm view mapterrain version` shows the new version
- [ ] Install test: `npx mapterrain@latest version --json` prints the new version

## Tag Naming Convention

Tags follow semver prefixed with `v`:

```
vX.0.0      # major
vX.Y.0      # minor (new features, no breaking changes)
vX.Y.Z      # patch (bug fixes only)
```

The `v` prefix is required — `release.yml` triggers on `push: tags: ['v*']`.
