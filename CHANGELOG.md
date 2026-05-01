# Changelog

All notable changes to Terrain are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Tracking 0.2 work — see `docs/release/0.2.md` for the full milestone.

### Added

- **Generated signal manifest export.** `docs/signals/manifest.json` is
  regenerated from `internal/signals.allSignalManifest` via
  `cmd/terrain-docs-gen`. `make docs-gen` writes; `make docs-verify` diffs.
- **CI hard-fail gate** on `make docs-verify` (extended ubuntu runner).
  Editing the manifest without committing the regenerated JSON now fails
  the PR. Carries the 0.1.2 scaffold to enforcement per the 0.2 plan
  (critical path item 1).
- **`TestManifestExport_StableEntriesHaveRuleURI`** tightens the manifest
  contract: status=stable requires a non-empty `RuleURI`. Experimental
  and planned entries may still leave it blank.
- **Eval framework adapter — Promptfoo.** `internal/airun.ParsePromptfooJSON`
  reads Promptfoo `--output` payloads (v3 nested + v4 flat shapes) into a
  normalised `EvalRunResult`. `TestSuiteSnapshot.EvalRuns` carries the
  envelope; per-case payloads decode via `airun.ParseEvalRunPayload`.
  Foundation for the runtime-aware AI detectors (aiCostRegression,
  aiHallucinationRate, aiRetrievalRegression) that land later in 0.2.

### Changed

- `package.json`, `extension/vscode/package.json`, `package-lock.json`
  bumped to `0.2.0` to mark the start of the 0.2 cycle.

## [0.1.2] — Truth-up & foundation

The deliberate "boring" release. No new headline features; instead, every
gap between what Terrain marketed and what the code actually delivered is
either closed or explicitly tagged. Schemas, signal vocabulary, and
distribution surfaces are locked so 0.2 can ship features against a stable
foundation. Per `docs/release/0.1.2.md`.

### Honest about what ships

- New: `docs/release/feature-status.md` is the canonical inventory of
  stable / experimental / planned features. Drift between marketing and
  code becomes a release blocker starting in 0.2.
- README: example CLI outputs are now framed explicitly as illustrative
  shape, not literal output. Three signals shown (`xfailAccumulation`,
  statistical ">10% failure rate" flaky detection, `0.91+` duplicate
  similarity) are explicitly tagged `[experimental]` or `[planned]`
  because the underlying detectors don't ship in 0.1.2.
- README: the "30 seconds" claim is now scoped to small-to-medium repos
  with realistic numbers for larger workspaces.
- `docs/legacy/`: every file now carries a strong **DEPRECATED — DO NOT
  USE FOR NEW WORK** banner pointing at current docs.
- `internal/convert/catalog.go`: 10 conversion directions tagged
  `GoNativeStateExperimental` per round 3 audit (Java, Python,
  TestCafe, Selenium families). `terrain convert` warns to stderr when
  invoked on an experimental direction.

### Distribution

- Goreleaser now builds five platforms instead of one: darwin/amd64,
  darwin/arm64, linux/amd64, linux/arm64, windows/amd64. Each is built
  on a matching CI runner because go-tree-sitter requires CGO and
  cannot cross-compile cleanly.
- Release archives, SBOMs, and checksums are signed via Sigstore
  keyless cosign. Signatures and certificates are uploaded with each
  artifact.
- npm postinstall (`bin/terrain-installer.js`) gains a best-effort
  cosign verifier: in 0.1.2 it warns on missing cosign, missing
  signature artifacts, or verification failure but does not block
  install. 0.2 makes this hard-fail unless
  `TERRAIN_INSTALLER_SKIP_VERIFY=1` is set.
- `.github/dependabot.yml`: gomod, github-actions, and the VS Code
  extension package are now tracked alongside the existing root-npm
  ecosystem. Tree-sitter grammar updates surface as PRs automatically.

### Schema & signal vocabulary

- `internal/signals/manifest.go` (new): single source of truth for all
  56 signal types. Status (stable / experimental / planned), default
  severity, confidence range, evidence sources, RuleID, RuleURI, and
  promotion plan are recorded for every entry.
  `TestManifest_MatchesSignalTypes` makes constant↔manifest drift a
  build failure.
- `internal/models/MaxSupportedMajorSchema = 1`. Snapshot reads now
  reject majors above the current binary's understanding via
  `ValidateSchemaVersion`.
- `docs/schema/COMPAT.md` (new): the public compatibility contract.
  Documents what is allowed at minor steps, what requires a major bump,
  and how the manifest's drift gates fit in.
- `docs/scoring-rubric.md` and `docs/health-grade-rubric.md` (new):
  every magic number behind risk-band assignment and Health Grade
  derivation is now extracted to a named constant and explained.

### Correctness & durability fixes

- `.gitignore` is now honoured during repository scanning. Vendored
  trees and generated artefacts the user has explicitly excluded are
  no longer walked.
- File cache is bounded: per-file 8 MB, total 256 MB. Files past the
  cap stream from disk on every read instead of failing the process.
- Worker-pool sizing capped at `min(GOMAXPROCS, 16)`.
- Framework detection probe size raised from 64 KB to 256 KB.
- `internal/metrics/metrics.go:Derive`, `internal/analyze/analyze.go:Build`,
  and `internal/insights/insights.go:Build` are now nil-safe; the
  adversarial test that previously swallowed their panics with
  `t.Logf("acceptable")` is now a strict contract test that fails on
  panic.

### CLI ergonomics

- `NO_COLOR`, `TERM=dumb`, and every common CI provider
  (GitHub Actions, GitLab, CircleCI, Buildkite, Jenkins, Azure
  Pipelines) now suppress progress output. Logs no longer get
  carriage-return garbage in CI.
- Did-you-mean suggestions on unknown commands. Levenshtein distance
  ≤2 gets you up to three suggestions; in-tree implementation, no new
  dependency.
- Exit codes documented as a 5-level scheme. `exitPolicyViolation`
  remains 2 for back-compat in 0.1.2; 0.2 splits it cleanly.
- `terrain doctor` and `terrain ai doctor` consolidation deferred to
  0.2 (the larger CLI restructure).

### Security & privacy

- `--base` git refs are validated against an allow-listed regex
  before being passed to `git diff`. Shell-injection payloads,
  reflog selectors (`@{-1}`), `--upload-pack=evil`, and whitespace
  are all rejected.
- Telemetry config and event log now ship 0o600; the parent
  `~/.terrain` directory ships 0o700.
- SARIF emission gains `--redact-paths`; absolute paths inside the
  repo are rewritten relative, paths outside collapse to bare
  basenames.
- `terrain serve` ships a security middleware: CSP, X-Frame-Options
  DENY, X-Content-Type-Options nosniff, Referrer-Policy no-referrer
  on every response. Origin/Referer validation rejects browser-driven
  cross-origin attacks against localhost. New `--host` flag warns
  when bound to a non-localhost address.

### CI & governance

- Multi-OS test matrix: ubuntu-latest, macos-latest, windows-latest.
  ubuntu remains the canonical runner with the race detector and full
  fixture suite; macos and windows run unit tests to catch
  platform-specific regressions before binaries ship.
- Determinism gate (`make test-determinism`) now runs in CI on every
  PR.
- New: `.github/CODEOWNERS`, `.github/pull_request_template.md`,
  `.husky/pre-commit` (blocks files >5 MB and binary-only extensions).
- `.nvmrc` strict-pinned to `22.11.0`.

### Removed

- `internal/plugin/` package (extension-point interfaces that were
  never wired into the engine). The only adopters were tests in the
  package itself. Detector contributors should read
  `docs/engineering/detector-architecture.md` for the actual in-tree
  registry pattern.

### Versioning

- npm package, `extension/vscode/package.json`, and
  `package-lock.json` all bumped to `0.1.2`. Git-tag/package.json
  drift is now a release-gate failure.

## 0.1.0 — Test System Intelligence Platform (2026-04-06)

Terrain 0.1.0 is the first public release of the Terrain test intelligence
platform. A ground-up rewrite of the analysis engine in Go, the legacy
JavaScript converter becomes one subsystem within a signal-first intelligence
platform that maps test suites, surfaces risk, and drives CI optimization —
all from a single statically-linked binary with zero runtime dependencies.

**83k lines of Go across 47 internal packages. 210 test files. 48 test
packages, all passing. Zero `go vet` warnings. Zero `gofmt` issues.**

### Core Analysis Pipeline

- 10-step deterministic pipeline: scan, policy, signals, ownership, runtime, risk, coverage, measurement, portfolio, snapshot
- Repository scanning with framework detection (17 frameworks across Go, JS/TS, Python, Java)
- Test file discovery, code unit extraction, import graph construction
- Signal-first architecture: every finding is a structured Signal with type, severity, confidence, evidence, and location
- Code surface inference: prompts, contexts, datasets, tool definitions, retrieval/RAG, agents, eval definitions
- Behavior surface derivation from API routes, event handlers, and state transitions
- Environment/device matrix analysis from CI configs and framework settings

### 18 Measurements Across 5 Posture Dimensions

| Dimension | Measurements |
|-----------|-------------|
| Health | flaky share, skip density, dead test share, slow test share |
| Coverage Depth | uncovered exports, weak assertion share, coverage breach share |
| Coverage Diversity | mock-heavy share, framework fragmentation, E2E concentration, E2E-only units, unit test coverage |
| Structural Risk | migration blocker density, deprecated pattern share, dynamic generation share |
| Operational Risk | policy violation density, legacy framework share, runtime budget breach share |

### Signal Detectors

- **Quality**: weak assertions, mock-heavy tests, untested exports, assertion-free tests, orphaned tests
- **Health**: slow tests, flaky tests, skipped tests, dead tests, unstable suites (runtime-backed)
- **Migration**: deprecated patterns, dynamic test generation, custom matchers, unsupported setup, framework fragmentation
- **Governance**: policy violations, legacy framework usage, runtime budget exceeded, AI safety
- **Structural**: phantom eval scenarios, blast-radius hotspots, coverage gap clusters

### CLI Commands (30+)

**Primary commands:**
- `terrain analyze` — full test system analysis with key findings, repo profile, risk posture
- `terrain insights` — prioritized health report with categorized findings and recommendations
- `terrain impact` — change-scope analysis: impacted units, tests, protection gaps, owners
- `terrain explain` — structured reasoning chains for any entity (test, unit, owner, scenario, selection)

**Supporting commands:**
- `terrain init` — detect data files and generate recommended analyze command
- `terrain summary` — executive summary with risk, trends, benchmark readiness
- `terrain focus` — prioritized next actions with top risk areas
- `terrain posture` — detailed posture breakdown with measurement evidence
- `terrain portfolio` — portfolio intelligence: cost, breadth, leverage, redundancy
- `terrain metrics` — aggregate metrics scorecard
- `terrain compare` — snapshot-to-snapshot trend tracking
- `terrain select-tests` — protective test set for a change
- `terrain pr` — PR/change-scoped analysis (markdown, comment, annotation output)
- `terrain show <entity> <id>` — drill into test, unit, owner, or finding
- `terrain migration <sub>` — readiness, blockers, or preview
- `terrain policy check` — evaluate local policy rules (exit 0/1/2 for CI)
- `terrain export benchmark` — privacy-safe JSON export
- `terrain serve` — local HTTP server with HTML report and JSON API

**AI / eval:**
- `terrain ai list` — list detected scenarios, prompts, datasets, eval files
- `terrain ai run` — execute eval scenarios with impact-based selection
- `terrain ai replay` — replay and verify a previous run artifact
- `terrain ai record` — save eval results as baseline
- `terrain ai baseline` — manage eval baselines (show, compare)
- `terrain ai doctor` — validate AI/eval setup

**Conversion / migration:**
- `terrain convert` — Go-native source test conversion (25 directions)
- `terrain convert-config` — framework config file conversion
- `terrain migrate` — project-wide migration with state tracking
- `terrain estimate` — migration complexity estimation
- `terrain status` / `terrain checklist` / `terrain doctor` / `terrain reset` — migration workflow
- `terrain list-conversions` / `terrain shorthands` / `terrain detect` — catalog and detection
- 50 shorthand aliases (e.g., `terrain cy2pw`, `terrain jest2vt`)

**Debug:**
- `terrain debug graph|coverage|fanout|duplicates|depgraph` — internal analysis inspection

### AI / Regular Test Parity

AI surfaces receive the same CI treatment as regular tests:

- Discovery: prompts, contexts, datasets, tool definitions, RAG pipelines, agents, eval definitions
- Impact selection: `terrain ai run --base main` selects only impacted eval scenarios
- Protection gaps: changed AI surfaces without eval coverage appear in `terrain impact` and `terrain pr`
- Policy enforcement: 7 AI-specific policy rules (`block_on_safety_failure`, `block_on_uncovered_context`, etc.)
- PR comments: AI Validation section in `terrain pr` output (markdown + text)
- GitHub Action: `terrain-ai.yml` template for AI CI gates
- Health insights: uncovered AI surfaces appear in `terrain insights`

### Structural Intelligence

Three features that use the dependency graph and surface model to produce recommendations no individual tool can generate:

- **"What to test next"**: ranks untested source files by import graph dependency count — files with more dependents create larger blind spots for change-scoped test selection
- **AI behavior impact chains**: detects files with multiple AI surface types where some are covered and others aren't — a change to the untested surface can alter downstream AI behavior undetected
- **Capability gap detection**: identifies AI capabilities with only positive/accuracy scenarios but no adversarial, safety, or robustness scenarios

### Impact Analysis

- Change-scope analysis against git diff with structural dependency tracing
- Protective test set selection with confidence scoring and reason chains
- Edge-case policy: fallback strategies, confidence adjustments, risk elevation
- Drill-down views: units, gaps, tests, owners, graph, selected
- Manual coverage overlay for untestable paths
- PR-scoped output: markdown, CI comment, GitHub annotations
- AI protection gaps: changed AI surfaces without eval coverage

### Dependency Graph Engine

- 5 reasoning engines: coverage, duplicates, fanout, redundancy, profile
- Edge-case detection (14 types) with policy recommendations
- Stability clustering for shared root-cause detection
- Environment/device matrix coverage analysis
- Language-aware fanout threshold (25, calibrated across Go/Python/JS/Java)

### Go-Native Conversion Runtime

- 25 conversion directions across 4 categories (E2E, unit JS, unit Java, unit Python)
- AST-based converters using tree-sitter for structural accuracy
- Semantic validation of converted output
- Config file conversion (Jest, Vitest, Cypress, Playwright, WebdriverIO, Mocha)
- Project-wide migration with dependency ordering, state tracking, resume/retry
- Confidence scoring per converted file

### Artifact Ingestion

- Runtime: JUnit XML and Jest JSON parsers with file-level metric aggregation
- Coverage: LCOV and Istanbul JSON parsers with code unit attribution
- Coverage by type: unit, integration, e2e run labeling
- Per-test coverage mapping
- Gauntlet AI eval artifact ingestion
- Auto-discovery of common artifact paths

### Reporting

- 14 report renderers (analyze, impact, insights, posture, metrics, portfolio, summary, focus, migration, policy, comparison, explain, impact drilldown, executive)
- HTML report with embedded charts
- SARIF output for IDE integration
- GitHub annotation output for CI
- Markdown PR comment output

### Snapshot and Comparison

- `terrain analyze --write-snapshot` — persist snapshots for trend tracking
- `terrain compare` — snapshot-to-snapshot comparison with signal deltas and risk band changes
- Automatic trend loading in summary and insights commands

### Ownership

- CODEOWNERS file parsing with glob pattern matching
- terrain.yaml ownership configuration
- Git history-based ownership inference
- Owner-scoped health and quality summaries
- Owner-filtered impact analysis

### Policy and Governance

- `.terrain/policy.yaml` rule definitions
- AI policy: block on safety failure, block on signal types
- Framework allowlists and denylists
- Runtime budget enforcement
- CI-friendly exit codes (0 = pass, 2 = violations)

### Packaging

- goreleaser config for multi-platform binaries (macOS, Linux, Windows; amd64, arm64)
- SBOM generation (CycloneDX, SPDX)
- Sigstore signing
- Homebrew tap (`pmclSF/terrain/mapterrain`)
- npm package (`mapterrain`) with platform-specific binary installation
- VS Code extension with sidebar views and commands
- Opt-in privacy-respecting telemetry (local only, no network)

---

## 0.0.1 — Signal-First Foundation (2026-04-03)

Internal milestone. Initial Go-native analysis engine with signal-first
architecture, replacing the V2 JavaScript converter.

### Core Analysis
- Repository scanning with framework detection (17 JS/TS/Java/Python/Go frameworks)
- Test file discovery and code unit extraction
- Signal-first architecture: every finding is a structured Signal with type, severity, evidence, and location
- Evidence model with strength (strong/moderate/weak), source, and confidence scoring

### Signal Detectors
- **Quality**: weak assertions, mock-heavy tests, untested exports, coverage threshold breaks
- **Migration**: deprecated patterns, dynamic test generation, custom matchers, unsupported setup, framework fragmentation
- **Governance**: policy violations, legacy framework usage, runtime budget exceeded
- **Health**: slow tests, flaky tests, skipped tests (runtime-backed)

### Risk Modeling
- Explainable risk engine with reliability, change, and speed dimensions
- Risk surfaces by file, directory, owner, and repository scope
- Heatmap model with directory and owner hotspots

### Migration Intelligence
- `terrain migration readiness` — readiness assessment with quality factors and area assessments
- `terrain migration blockers` — blockers by type and area with representative examples
- `terrain migration preview` — file-level and scope-level migration difficulty preview

### VS Code Extension
- Sidebar views: Overview, Health, Quality, Migration, Review
- TreeDataProvider implementations over CLI JSON output

### Packaging
- goreleaser config for multi-platform binaries
- `terrain version` with build metadata
