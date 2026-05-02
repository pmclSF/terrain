# Compatibility Statement

What Terrain runs on, and what versions of upstream tools it
understands.

## Host platform

Pre-built binaries (downloaded from GitHub Releases or via the npm
installer) ship for:

| OS | Architectures | Tier |
|----|---------------|------|
| Linux | amd64, arm64 | Tier 1 (full CI matrix) |
| macOS | amd64, arm64 | Tier 1 (full CI matrix) |
| Windows | amd64 | Tier 1 (full CI matrix) |
| Windows arm64 | — | Not built (planned 0.3) |

Tier 1 means CI runs `go test ./...` on every PR before merge.
Source builds work on any platform Go 1.23+ supports.

## Build-time

| Tool | Minimum |
|------|---------|
| Go | 1.23 |
| Node.js (for npm install + extension build) | 22.x |
| C compiler (for tree-sitter parsers) | gcc/clang installed |
| Make | 3.81+ (BSD or GNU) |

## Frameworks understood

### Test frameworks

Terrain's `analyze` command structurally models tests for the
following frameworks. "Tier" reflects fixture coverage in the
calibration corpus and conversion-direction A-grade ratings.

| Framework | Language | Tier |
|-----------|----------|------|
| Jest | JS/TS | Tier 1 |
| Vitest | JS/TS | Tier 1 |
| Mocha | JS/TS | Tier 1 |
| Playwright | JS/TS | Tier 1 |
| Cypress | JS/TS | Tier 1 |
| pytest | Python | Tier 1 |
| Go testing (`go test`) | Go | Tier 1 |
| JUnit 5 | Java | Tier 2 |
| RSpec | Ruby | Tier 2 |
| Karma | JS | Tier 2 |
| Jasmine | JS | Tier 2 |
| Tap | JS | Tier 2 |
| AVA | JS | Tier 2 |
| WebdriverIO | JS/TS | Tier 2 |
| Puppeteer | JS/TS | Tier 2 |

Tier 1 = stable detector + structural model + at least one
calibration fixture. Tier 2 = detected and counted, but with
shallower structural modeling.

### AI eval frameworks

| Framework | Versions |
|-----------|----------|
| Promptfoo | v3 (nested results), v4+ (flat results); both shapes accepted |
| DeepEval | testCases shape (older) + runId shape (1.x) |
| Ragas | results / evaluation_results / scores shapes; ≥0.1.0 modern metrics |

Adapter behavior: each framework's per-case score, cost, and
failure-reason data flow into the snapshot's `EvalRuns` envelope
and feed `aiCostRegression` / `aiHallucinationRate` /
`aiRetrievalRegression`.

### CI providers

Terrain's PR analysis works on any CI that can run a binary and
read git history. We document GitHub Actions templates in
[`README.md`](../README.md). Per-provider integration guides for
GitLab CI, CircleCI, Jenkins, and pre-commit hooks are tracked in
the 0.3 backlog.

## Snapshot schema compatibility

Snapshots written by version 0.X.Y are forward-compatible with all
later 0.X.Z patches. Cross-MAJOR (0.x → 1.x in the future) requires
explicit migration; the engine rejects unknown major versions.

Current schema version: see `models.SnapshotSchemaVersion` (also
surfaced via `terrain version --json`).

The full version history is in
[`docs/schema/COMPAT.md`](schema/COMPAT.md).

## CLI / output stability

See [`docs/versioning.md`](versioning.md) for the semver contract,
including which surface changes are breaking vs behavior-only vs
bug-fix.
