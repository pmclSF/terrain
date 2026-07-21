# Compatibility

What Terrain runs on, and what versions of upstream tools it understands.

## Host platform

Pre-built binaries ship for:

| OS | Architectures | Tier |
|---|---|---|
| Linux | amd64, arm64 | Tier 1 — extended CI (race detector, determinism, post-publish smoke) |
| macOS | amd64, arm64 | Tier 1 — unit-test parity in CI |
| Windows | amd64 | Tier 1 — unit-test parity in CI |
| Windows arm64 | — | Not built |

Tier 1 means a pre-built binary ships and `go test ./...` runs in CI on every PR. Linux additionally runs the race detector and a byte-identical determinism check; the published linux/amd64 archive is smoke-tested post-publish (other architectures will join that smoke as it becomes feasible). Source builds work on any platform Go 1.23+ supports.

## Build-time

| Tool | Minimum |
|------|---------|
| Go | 1.23 |
| Node.js (for npm install + extension build) | 22.x |
| C compiler (for tree-sitter parsers) | gcc/clang installed |
| Make | 3.81+ (BSD or GNU) |

## Test frameworks

| Framework | Language | Tier |
|---|---|---|
| Jest, Vitest, Mocha, Playwright, Cypress | JS/TS | Tier 1 |
| pytest | Python | Tier 1 |
| Go testing (`go test`) | Go | Tier 1 |
| JUnit 5 | Java | Tier 2 |
| RSpec | Ruby | Tier 2 |
| Karma, Jasmine, Tap, AVA, WebdriverIO, Puppeteer | JS/TS | Tier 2 |

Tier 1 = stable detector + structural model + at least one recall-regression fixture. Tier 2 = detected and counted with shallower structural modeling. Tier 2 framework support does not imply full source-language analysis: RSpec/Ruby artifacts are detected and counted, but Ruby source is not analyzed in 0.4.0.

## AI eval frameworks

| Framework | Versions |
|---|---|
| Promptfoo | v3 (nested) + v4+ (flat) results |
| DeepEval | testCases shape (older) + runId shape (1.x) |
| Ragas | results / evaluation_results / scores; ≥0.1.0 modern metrics |
| Great Expectations | Validation Result JSON with `results[]` and optional `statistics` |

Per-case score and failure-reason data flow into the snapshot's `EvalRuns` envelope. Axis-specific detectors (`aiCostRegression`, `aiHallucinationRate`, `aiRetrievalRegression`) fire only when the artifact provides the cost, hallucination, or retrieval metadata those detectors require.

## Schemas + ORMs

Terrain extracts schema definitions from these sources, then correlates their fields with prompt-template references (the schema↔prompt drift detector) across files and languages:

| Source | Notes |
|---|---|
| Postgres, MySQL | DDL + `CREATE TABLE` extraction |
| Pydantic | Python model classes |
| TypeScript types | `type` and `interface` |
| sqlc | Go-from-SQL codegen |
| gorm | Go struct tags |
| Prisma | `schema.prisma` |
| sqlalchemy | Python declarative + classical mappings |

## Data + ML pipelines

| Tool | Use |
|---|---|
| dbt | model + source graph |
| Airflow, Prefect | DAG / flow + task graph |
| MLflow | experiment + run + model registry |
| Weights & Biases | run + artifact registry |

## CI providers

Any CI that runs a binary and can read git history. GitHub Actions templates ship in [`docs/examples/gate/`](examples/gate/README.md). Dedicated guides for GitLab CI, CircleCI, Jenkins, and pre-commit hooks will follow as adopters request them.

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
