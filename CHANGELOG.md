# Changelog

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

### Impact Analysis

- Change-scope analysis against git diff with structural dependency tracing
- Protective test set selection with confidence scoring and reason chains
- Edge-case policy: fallback strategies, confidence adjustments, risk elevation
- Drill-down views: units, gaps, tests, owners, graph, selected
- Manual coverage overlay for untestable paths
- PR-scoped output: markdown, CI comment, GitHub annotations

### Dependency Graph Engine

- 5 reasoning engines: coverage, duplicates, fanout, redundancy, profile
- Edge-case detection (14 types) with policy recommendations
- Stability clustering for shared root-cause detection
- Environment/device matrix coverage analysis

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
