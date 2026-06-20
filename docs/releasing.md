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
    │           ├── make npm-release-verify
    │           │     ├── npm ci
    │           │     ├── format:check
    │           │     ├── lint
    │           │     └── verify-pack.js
    │           │           ├── npm pack → install in temp dir
    │           │           ├── CLI smoke (`terrain`, `mapterrain`)
    │           │           └── Conversion smoke (`terrain convert`, `terrain migrate`)
    │           └── make extension-verify
    │                 ├── npm --prefix extension/vscode ci
    │                 ├── npm --prefix extension/vscode run compile
    │                 └── npm --prefix extension/vscode test
    ├── go-release-build job (needs: verify):
    │     ├── goreleaser build --clean (matrixed by OS)
    │     └── Archive binaries with README/LICENSE, SBOMs, signatures
    ├── go-release-publish job (needs: go-release-build):
    │     ├── Create one GitHub Release with binaries + SBOMs + checksums
    │     └── Attach SLSA provenance attestations
    ├── release-smoke job (needs: go-release-publish):
    │     └── Download representative published archives and verify version
    └── npm-release job (needs: verify + go-release-publish + release-smoke):
          ├── npm ci
          └── npm publish --provenance (NPM_TOKEN secret)
```

A single workflow (`release.yml`) handles the release pipeline: verify → GitHub release → smoke test → npm release. The Homebrew tap update is handled by `.github/workflows/homebrew-update.yml` after the GitHub release is published. The npm package publishes only after representative GitHub release archives pass smoke tests, because the `mapterrain` npm package installs the Go CLI from those tagged assets.

### Go Binary Release

The `go-release-build` job uses [GoReleaser](https://goreleaser.com/) to build
multi-platform binaries (Linux/macOS amd64+arm64, Windows amd64). The workflow then
archives each binary with `README.md` and `LICENSE`, generates per-archive SBOMs,
and signs the artifacts. The `go-release-publish` job merges matrix artifacts,
recomputes a single `checksums.txt`, signs it, attaches SLSA provenance, and
creates the GitHub Release. Build configuration lives in `.goreleaser.yaml`;
Homebrew publishing is intentionally handled by `homebrew-update.yml`.

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
