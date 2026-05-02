# Terrain

**Map your test terrain.** Understand your test system in 30 seconds.

```bash
# Homebrew
brew install pmclSF/terrain/mapterrain

# npm
npm install -g mapterrain

cd your-repo
terrain analyze
```

That's it. No config, no setup, no test execution required.

> **New here?** Read the [Quickstart Guide](docs/quickstart.md) to understand your first report in 5 minutes.

---

Terrain is a test system intelligence platform. It reads your repository — test code, source structure, coverage data, runtime artifacts, ownership files, and local policy — and builds a structural model of how your tests relate to your code. From that model it surfaces risk, quality gaps, redundancy, fragile dependencies, and migration readiness, all without running a single test.

The core idea: every codebase has a *test terrain* — the shape of its testing infrastructure, the density of coverage across areas, the hidden fault lines where a fixture change cascades into thousands of tests. Terrain makes that shape visible and navigable so you can make informed decisions about what to test, what to fix, and where to invest.

## What "Test Terrain" Means

Most teams know what tests they have. Few teams understand the *terrain* underneath:

- Which source files have no structural test coverage?
- Which shared fixtures fan out to thousands of tests, making any change to them a blast-radius problem?
- Which test clusters are near-duplicates burning CI time?
- Which areas have tests but weak assertions that wouldn't catch a real regression?
- When you change `auth/session.ts`, which 41 of your 18,000 tests actually matter?

Test terrain is the structural topology of your test system — the dependency graph, the coverage landscape, the duplication clusters, the fanout hotspots, the skip debt. Terrain maps it.

## Problems Terrain Solves

**"We don't know the state of our test system."** Teams inherit test suites they didn't write. Terrain gives a baseline in seconds: framework mix, coverage confidence, duplication, risk posture.

**"CI takes too long and we don't know what to cut."** Terrain identifies redundant tests, high-fanout fixtures, and confidence-based test selection — showing where CI time is wasted and what can be safely reduced.

**"We changed auth code — what tests should we worry about?"** Terrain traces your change through the import graph and structural dependencies, returning the impacted tests ranked by confidence, with reason chains explaining each selection.

**"A tool flagged something but won't explain why."** Every Terrain finding carries an evidence chain. `terrain explain` shows exactly what signals, dependency paths, and scoring rules produced each decision.

**"We're migrating frameworks and need to know what's blocking us."** Migration readiness, blockers by type, and preview-scoped difficulty assessment — all derived from static analysis.

## The Four Canonical Workflows

Terrain is organized around four questions. Everything else is a supporting view.

```
terrain analyze     "What is the state of our test system?"
terrain insights    "What should we fix in our test system?"
terrain impact      "What validations matter for this change?"
terrain explain     "Why did Terrain make this decision?"
```

> **About the example outputs below.** The CLI dumps in this section illustrate the *shape* of Terrain's reports on a large pandas-style repository — they are not literal output from a single live run. A few specific signals shown (`xfailAccumulation` age, statistical flaky-test failure rates, the `0.91+` duplicate similarity threshold) are marked `[experimental]` or `[planned]` in 0.2.0; see [docs/release/feature-status.md](docs/release/feature-status.md) for what's stable, what's experimental, and what's planned. The headline "30 seconds" promise refers to small-to-medium repos (≤ 1,000 test files) on commodity hardware; expect 5–15 seconds on a typical service repo and longer on monorepos.

### 1. Analyze — understand the test system

```bash
terrain analyze
```

```
Terrain — Test Suite Analysis
============================================================

  conftest.py fixture fans out to 3,100 tests — any change retriggers the frame/ suite.

Key Findings
------------------------------------------------------------
  1. [HIGH] 23 source files (18%) have low structural coverage
  2. [HIGH] 8 duplicate test clusters with 0.91+ similarity
  3. [MEDIUM] 34 xfail markers older than 180 days

Repository Profile
------------------------------------------------------------
  Test volume:          very large
  CI pressure:          high
  Coverage confidence:  medium
  Redundancy level:     medium
  Fanout burden:        high

Validation Inventory
------------------------------------------------------------
  Test files:     1,047
  Test cases:     52,341
  Frameworks:
    pytest                1,047 files [unit]

Risk Posture
------------------------------------------------------------
  health:              MODERATE
  coverage_depth:      ELEVATED
  coverage_diversity:  STRONG
  structural_risk:     STRONG
  operational_risk:    STRONG

Next steps:
  terrain insights            prioritized actions and recommendations
  terrain impact              what tests matter for this change?
```

### 2. Insights — find what to improve

```bash
terrain insights
```

```
Terrain — Test System Health Report
============================================================

Health Grade: C

Reliability Problems (2)
  [HIGH] 12 flaky tests with >10% failure rate
  [MEDIUM] 34 skipped tests consuming CI resources

Coverage Debt (2)
  [HIGH] 23 source files (18%) have low structural coverage
  [MEDIUM] conftest.py fixture fans out to 3,100 tests

Recommended Actions
  1. [reliability] Quarantine 12 flaky tests
     why: Flaky tests block unrelated PRs and erode CI trust.
  2. [coverage] Add tests for 23 uncovered source files
     why: Changes in uncovered files cannot trigger test selection.
  3. [optimization] Consolidate 8 duplicate test clusters
     why: 0.91+ similarity — redundant CI cost with no coverage benefit.
```

### 3. Impact — understand what a change affects

```bash
terrain impact --base main
```

```
Terrain Impact Analysis
============================================================

Summary: 3 file(s) changed, 41 test(s) relevant. Posture: needs_attention.

Impacted tests:          127 of 52,341 total
Coverage confidence:     High

Recommended Tests (41)
------------------------------------------------------------
  tests/groupby/test_groupby.py [exact]
    Covers: groupby.py:GroupBy.aggregate
  tests/groupby/test_apply.py [exact]
    Covers: groupby.py:GroupBy.apply
  tests/resample/test_base.py [inferred]
    Reached via shared fixture path
  ...and 38 more

Protection Gaps
------------------------------------------------------------
  [medium] pandas/core/groupby/ops.py — no covering tests found

Next steps:
  terrain impact --show tests      full test list
  terrain impact --show gaps       all protection gaps
```

### 4. Explain — understand why

```bash
terrain explain tests/io/json/test_pandas.py
```

```
Test File: tests/io/json/test_pandas.py
Framework: pytest
Tests: 84    Assertions: 312

Signals (3):
  [high]   networkDependency: 12 tests use @pytest.mark.network — flaky in CI
  [medium] weakAssertion: 8 bare assert statements without descriptive messages
  [low]    xfailAccumulation: 3 xfail markers older than 180 days

Next steps:
  terrain explain selection       explain overall test selection strategy
  terrain impact --show tests     see all impacted tests
```

See [Canonical User Journeys](docs/product/canonical-user-journeys.md) for the full workflow specification and [example outputs](docs/examples/) for detailed report samples.

## Product Philosophy

**Inference first.** Terrain reads code. It parses imports, detects frameworks, resolves coverage relationships, and builds dependency graphs from what already exists in the repository. No annotations, no test tagging, no SDK integration required.

**Zero-config by default.** `terrain analyze` works on any repository with test files. Coverage data, runtime artifacts, ownership files, and policy rules are optional inputs that enrich the model — but the core analysis requires nothing beyond the code itself.

**Explainability over magic.** Every finding carries evidence: which signal type, what confidence level, what dependency path, what scoring rule. `terrain explain` exposes the full reasoning chain behind any decision. Teams should never wonder *why* Terrain said something.

**Conservative under uncertainty.** When Terrain encounters ambiguity — a dependency path with low confidence, a file that might or might not be a test — it flags the uncertainty rather than guessing. Impact analysis uses fallback policies with explicit confidence penalties rather than silently expanding scope.

**System health, not individual productivity.** Terrain measures the test system. It never attributes quality to individual developers. Ownership information is used for routing and triage, not scoring.

## What Terrain Is Not

It's worth stating what Terrain *doesn't* try to do, because the test-tooling space has a lot of overlapping vendors and the boundaries matter.

- **Not a test runner.** Terrain doesn't execute your tests. It analyzes the test system around them. Pair it with `jest`, `pytest`, `go test`, your existing CI runner — Terrain reads the artifacts those produce, it doesn't replace them.
- **Not a coverage tool.** Terrain ingests coverage reports if you have them and uses them as evidence, but it doesn't instrument code or compute coverage itself. Bring coverage from `c8`, `istanbul`, `coverage.py`, `gcov` — Terrain is the layer that turns coverage into structural insight.
- **Not a static analyzer for application code.** Terrain inspects *test* code structure (assertions, mocks, framework patterns, scenario coverage). Tools like Sonar, Semgrep, and CodeQL stay better-positioned for source-side bug-finding; Terrain doesn't compete.
- **Not an LLM eval framework.** Terrain understands AI surfaces (prompts, scenarios, RAG pipelines) and the eval *artifacts* that promptfoo / DeepEval / Ragas produce, but it doesn't run the evals itself. Use those tools to execute; use Terrain to analyze what they produce in CI.
- **Not a test-flake whack-a-mole tool.** Terrain reports flakiness as a signal among many. If your only need is "rerun flaky tests until they pass", point-tools like `pytest-rerunfailures` or `jest-circus` ship that directly.
- **Not a developer-productivity dashboard.** Terrain measures the test system, not the people writing tests. It deliberately produces no leaderboards, no per-developer metrics, no "engineer productivity" rankings. Ownership data is used for routing, not scoring.
- **Not a service.** Terrain runs locally and in your CI. There is no SaaS offering, no telemetry sent off your infrastructure, no account required. Reports stay where you produce them.

If you're evaluating Terrain against another tool and the boundary isn't obvious, please open an issue — we'll write the comparison entry under `docs/compare/`.

## Who Uses Terrain

Terrain is framework-agnostic and language-aware. The same analysis model applies across:

- **Frontend teams** — React/Vue component tests, Playwright/Cypress E2E suites, Vitest/Jest unit tests
- **Backend teams** — Go test suites, pytest collections, JUnit hierarchies, integration test infrastructure
- **Mobile teams** — cross-platform test suites with standard test frameworks
- **QA / SDET** — test portfolio management, coverage gap analysis, migration planning across frameworks
- **SRE / Platform** — CI optimization, test selection for pipelines, policy enforcement
- **AI / ML evaluation** — evaluation suite structure, benchmark test management, coverage across model behaviors

The structural model is the same. The signals and recommendations adapt to the framework and test patterns detected.

## AI Testing in CI

Terrain gives AI components the same CI safety net as regular tests:

- **Surface discovery** — automatically detects prompts, contexts, datasets, tool definitions, RAG pipelines, and eval scenarios in your code
- **Impact-scoped selection** — `terrain ai run --base main` runs only the eval scenarios affected by your change
- **Protection gaps** — `terrain pr` flags changed AI surfaces that have no eval scenario covering them
- **Policy enforcement** — block PRs that modify uncovered AI surfaces, regress accuracy, or trigger safety failures
- **GitHub Action** — drop-in `terrain-ai.yml` workflow template for AI CI gates

The same structural graph that powers test selection for regular code also traces AI surface dependencies, so a change to a prompt template triggers the right eval scenarios automatically.

## How CI Optimization Emerges

Terrain does not start with CI optimization. It starts with understanding.

When you run `terrain analyze`, Terrain builds a structural model: which tests exist, which source files they cover, how they depend on shared fixtures, where duplication lives. From that model, CI optimization *emerges*:

- **Test selection** — `terrain impact` traces a diff through the dependency graph and returns only the tests that structurally matter, ranked by confidence. This is not a heuristic skip list — it is a graph traversal with evidence.
- **Redundancy reduction** — `terrain insights` surfaces duplicate test clusters. Removing or consolidating them directly reduces CI time without reducing coverage.
- **Fanout control** — High-fanout fixtures that trigger thousands of tests on any change are identified and prioritized for splitting.
- **Confidence-based runs** — Impact analysis assigns confidence scores. CI pipelines can run high-confidence tests immediately and defer low-confidence tests to nightly runs.
- **What to test next** — `terrain insights` ranks untested source files by dependency count, telling you which test to write first for maximum impact.

The result is faster CI that comes from *understanding the test system*, not from skipping tests and hoping for the best.

## Installation

### Homebrew (macOS and Linux)

```bash
brew install pmclSF/terrain/mapterrain
```

After the first install, you can also tap once and use the short formula name:

```bash
brew tap pmclSF/terrain
brew install mapterrain
```

### npm

```bash
npm install -g mapterrain
```

### Go install

```bash
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

Requires Go 1.23 or later.

### Pre-built binaries

Download the appropriate binary for your platform from [GitHub Releases](https://github.com/pmclSF/terrain/releases), then:

```bash
chmod +x terrain
sudo mv terrain /usr/local/bin/
```

Binaries are available for macOS (amd64 + arm64), Linux (amd64 + arm64), and Windows (amd64).

### Build from source

```bash
git clone https://github.com/pmclSF/terrain.git
cd terrain
go build -o terrain ./cmd/terrain
```

### Verify installation

```bash
terrain --version
```

## Quick Start

```bash
# Detect coverage/runtime data paths (recommended first step)
terrain init

# Analyze the current repository
terrain analyze

# JSON output for any command
terrain analyze --json

# See what a change affects
terrain impact --base main

# Get prioritized recommendations
terrain insights

# Drill into a specific test, code unit, owner, or finding
terrain explain src/auth/login.test.ts
```

## GitHub Actions templates

Drop one of these into `.github/workflows/terrain.yml` and you're done.

### Minimal — analyze on every PR

```yaml
name: terrain
on:
  pull_request:

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - uses: actions/setup-node@v6
        with:
          node-version: '22.x'

      - run: npm install -g mapterrain
      - run: terrain analyze --root . --json > terrain-report.json
      - run: terrain impact --base origin/main --root .

      - uses: actions/upload-artifact@v4
        with:
          name: terrain-report
          path: terrain-report.json
```

### Strict — block on Critical / High signals

```yaml
name: terrain-gate
on:
  pull_request:

jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - run: |
          curl -L https://github.com/pmclSF/terrain/releases/latest/download/terrain_linux_amd64.tar.gz \
            | tar -xz

      - run: ./terrain analyze --root . --json > terrain.json

      # Fail the job if any Critical or High severity signals are
      # present in the analysis. Distinct exit codes per docs/cli-spec.md.
      - run: |
          jq -e '.signals | map(select(.severity == "critical" or .severity == "high")) | length == 0' terrain.json
```

### AI-aware — gate on AI-domain Criticals only

```yaml
name: terrain-ai-gate
on:
  pull_request:
    paths:
      - '**/*.py'
      - '**/*.js'
      - '**/*.ts'
      - '**/.terrain/**'
      - '**/promptfoo*.yaml'
      - '**/eval*.yaml'

jobs:
  ai-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - run: npm install -g mapterrain
      - run: terrain ai list --root . --json > ai-inventory.json

      # Fail only on aiHardcodedAPIKey or any AI-domain Critical.
      - run: |
          terrain analyze --root . --json |
            jq -e '[.signals[] | select(.category == "ai" and .severity == "critical")] | length == 0'

      - uses: actions/upload-artifact@v4
        with:
          name: ai-inventory
          path: ai-inventory.json
```

## Commands

### Primary commands

| Command | Question |
|---------|----------|
| `terrain analyze` | What is the state of our test system? |
| `terrain insights` | What should we fix in our test system? |
| `terrain impact` | What validations matter for this change? |
| `terrain explain <target>` | Why did Terrain make this decision? |

### Supporting commands

| Command | Purpose |
|---------|---------|
| `terrain init` | Detect data files and print recommended analyze command |
| `terrain summary` | Executive summary with risk, trends, benchmark readiness |
| `terrain focus` | Prioritized next actions |
| `terrain posture` | Detailed posture breakdown with measurement evidence |
| `terrain portfolio` | Portfolio intelligence: cost, breadth, leverage, redundancy |
| `terrain metrics` | Aggregate metrics scorecard |
| `terrain compare` | Compare two snapshots for trend tracking |
| `terrain select-tests` | Recommend protective test set for a change |
| `terrain pr` | PR/change-scoped analysis |
| `terrain show <type> <id>` | Drill into test, unit, owner, or finding |
| `terrain migration <sub>` | Migration readiness, blockers, or preview |
| `terrain policy check` | Evaluate local policy rules |
| `terrain export benchmark` | Privacy-safe JSON export for benchmarking |
| `terrain serve` | Local HTTP server with HTML report and JSON API |

### AI / eval

| Command | Purpose |
|---------|---------|
| `terrain ai list` | List detected scenarios, prompts, datasets, eval files |
| `terrain ai doctor` | Validate AI/eval setup and surface configuration issues |
| `terrain ai run` | Execute eval scenarios and collect results |
| `terrain ai replay` | Replay and verify a previous eval run artifact |
| `terrain ai record` | Record eval results as a baseline snapshot |
| `terrain ai baseline` | Manage eval baselines (show, compare) |

### Conversion / migration

| Command | Purpose |
|---------|---------|
| `terrain convert <source>` | Go-native test conversion (25 directions) |
| `terrain convert-config <source>` | Convert framework config files |
| `terrain migrate <dir>` | Project-wide migration with state tracking |
| `terrain estimate <dir>` | Estimate migration complexity |
| `terrain status` | Show migration progress |
| `terrain checklist` | Generate migration checklist |
| `terrain doctor [path]` | Run migration diagnostics |
| `terrain reset` | Clear migration state |
| `terrain list-conversions` | List supported conversion directions |
| `terrain shorthands` | List shorthand aliases (e.g., `cy2pw`, `jest2vt`) |
| `terrain detect <file-or-dir>` | Detect dominant framework |

### Advanced / debug

| Command | Purpose |
|---------|---------|
| `terrain debug graph` | Dependency graph statistics |
| `terrain debug coverage` | Structural coverage analysis |
| `terrain debug fanout` | High-fanout node analysis |
| `terrain debug duplicates` | Duplicate test cluster analysis |
| `terrain debug depgraph` | Full dependency graph analysis (all engines) |

Repository-scoped commands support `--root PATH`, and machine-readable commands support `--json`. Most analysis commands support `--verbose` for additional detail. Run `terrain <command> --help` for full flag documentation.

## Architecture Overview

Terrain is built around a signal-first architecture:

```
Repository scan  →  Signal detection  →  Risk modeling  →  Reporting
     │                    │                    │               │
  test files         framework-specific    explainable     human-readable
  source files       pattern detectors     risk scoring    + JSON output
  coverage data      quality signals       with evidence
  runtime artifacts  health signals        chains
  ownership files    migration signals
  policy rules       governance signals
```

- **Signals** are the core abstraction — every finding is a structured signal with type, severity, confidence, evidence, and location
- **Snapshots** (`TestSuiteSnapshot`) are the canonical serialized artifact — the complete structural model of a test system at a point in time
- **Risk surfaces** are derived from signals with explainable scoring across dimensions (quality, reliability, speed, governance)
- **Dependency graphs** model import relationships, fixture fanout, and structural coverage
- **Reports** synthesize signals, risk, trends, and benchmark readiness into actionable output

```
cmd/terrain/     CLI — canonical surface (analyze, report, migrate,
                 convert, posture, doctor, ai, serve, version, help)
                 plus legacy aliases retained through 0.2.x
internal/        47 Go packages covering analysis, signals, risk,
                 impact, depgraph, measurement, reporting, and more
```

See [DESIGN.md](DESIGN.md) for the full architecture overview and package map, [docs/architecture/](docs/architecture/) for detailed design documents, and [docs/json-schema.md](docs/json-schema.md) for JSON output structure.

## Snapshot Workflow

Terrain supports local snapshot history for trend tracking:

```bash
# Save a snapshot
terrain analyze --write-snapshot

# Later, save another snapshot
terrain analyze --write-snapshot

# Compare the two most recent snapshots
terrain compare

# Executive summary automatically includes trend highlights
terrain summary
```

Snapshots are stored in `.terrain/snapshots/` as timestamped JSON files.

## Policy

Define local policy rules in `.terrain/policy.yaml`:

```yaml
rules:
  disallow_skipped_tests: true
  max_weak_assertions: 10
  max_mock_heavy_tests: 5
```

Then check compliance:

```bash
terrain policy check        # human-readable output
terrain policy check --json # JSON output for CI
```

Exit code 0 = pass, 2 = violations found, 1 = error.

## Documentation

- [Quickstart Guide](docs/quickstart.md) — understand your first report in 5 minutes
- [CLI Specification](docs/cli-spec.md) — full command and flag reference
- [Example Reports](docs/examples/) — analyze, impact, insights, explain output samples
- [Canonical User Journeys](docs/product/canonical-user-journeys.md) — primary workflows and expected outcomes
- [Signal Model](docs/signal-model.md) — the core signal abstraction
- [Glossary](docs/glossary.md) — Terrain-specific vocabulary in one page
- [Architecture](docs/architecture/) — design documents and technical specifications
- [Versioning Policy](docs/versioning.md) — what's a breaking change vs behavior change vs bug fix
- [Compatibility](docs/compatibility.md) — supported OSes, Go versions, frameworks
- [Integrations](docs/integrations/) — Promptfoo / DeepEval / Ragas wiring guides
- [Feature Status](docs/release/feature-status.md) — what's stable, experimental, or planned in the current release
- [CHANGELOG](CHANGELOG.md) — release history and per-version changes
- [Security](SECURITY.md) — supported versions and vulnerability disclosure
- [Code of Conduct](CODE_OF_CONDUCT.md) — community standards
- [Contributing](CONTRIBUTING.md) — how to build, test, and extend Terrain

## Development

```bash
# Build
go build -o terrain ./cmd/terrain

# Test
go test ./cmd/... ./internal/...

# Full release verification
make release-verify
```

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
