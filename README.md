# Terrain

> **Pre-flight checks for AI/ML systems and the tests around them. Locally, no API key required.**
>
> Terrain is a static analyzer that treats unit tests, integration tests, e2e tests, and AI/ML evals as one dependency graph. It catches prompt-schema drift, train/test leakage, eval coverage gaps, model deprecations, framework-migration blockers, untested exports, and the cross-language cause chains general-purpose tools miss — locally, deterministically, on every PR.

**Five personas, one graph:**

- **Frontend developer** — find out before merge if a UI change breaks a downstream AI contract.
- **Backend / platform engineer** — see which models are affected by a schema or API change.
- **Classical ML engineer** — train/test integrity, drift, fairness regressions, performance deltas.
- **LLM engineer** — scenario-based eval regression across prompts, RAG components, and models.
- **Senior decision-maker** — public per-rule readiness cards and reproducible benchmarks.

Two commands an adopter learns first:

```bash
terrain analyze         # What's the state of our AI + test system?
terrain report pr       # What does this change put at risk?
```

Everything else is a deeper view *off* this primary workflow.

## Install

```bash
brew install pmclSF/terrain/mapterrain
cd your-repo
terrain analyze
```

No config and no test execution are required for the basic scan. Stronger findings (runtime health, eval regression, policy enforcement) are unlocked by optional artifacts; degrade gracefully when absent.

Other install paths (npm, `go install`, pre-built binaries, build from source) and the cosign-verification details are in the [Installation](#installation) section below.

## Trust & Data Handling

Terrain runs entirely on your machine. Zero outbound network calls in the default configuration; no API keys, no SaaS, no telemetry. Verifiable with `terrain --print-network`. For enterprise review, see [SECURITY-DATA-HANDLING.md](SECURITY-DATA-HANDLING.md).

> **New here?** Read the [Quickstart Guide](docs/quickstart.md) to understand your first report in 5 minutes.
> **Going deeper?** See [`docs/PRODUCT.md`](docs/PRODUCT.md) for the product reference and [`docs/OVERVIEW.md`](docs/OVERVIEW.md) for the evaluator-focused summary.
> **Adopting in CI?** See [What 0.2 is and isn't](#what-02-is-and-isnt) below first.

---

Terrain sits one layer above your test runners. Pytest, jest, Go test, Playwright, and Promptfoo continue to execute; Terrain reads what they produce, models the AI surface alongside the test surface, and gates pull requests on AI/ML risks, test-system regressions, and cross-language drift that general-purpose tools don't catch.

Common drift it catches: prompts referencing renamed schema fields, train/test leakage, eval coverage gaps, deprecated model IDs lingering in production paths, hardcoded API keys in source, framework-migration blockers, untested exports, and weak assertions.

## What Terrain Maps

Most teams know what tests they have. Few teams understand the *terrain* underneath:

- Which AI surfaces (prompts, agents, tools, contexts) have no eval scenario covering them?
- Which prompt templates reference schema fields that just got renamed in a different language?
- Which source files have no structural test coverage?
- Which shared fixtures fan out to thousands of tests, making any change a blast-radius problem?
- Which test clusters are near-duplicates burning CI time?
- When you change `auth/session.ts`, which 41 of your 18,000 tests actually matter?

Terrain maps the dependency graph, coverage landscape, duplication clusters, fanout hotspots, skip debt, AI surfaces, and cross-language edges — and turns each into a structured finding.

**Under the hood:**

- **Cross-language graph** — TS/JS ↔ Python/Go/Java edges via OpenAPI, tRPC, gRPC, GraphQL, and HTTP-route inference
- **Schema awareness** — Postgres, MySQL, Pydantic, TypeScript types, sqlc, gorm, prisma, sqlalchemy
- **Pipeline awareness** — dbt, Airflow, Prefect
- **ML registry awareness** — MLflow, W&B
- **Framework migration** — Jest ↔ Vitest, JUnit 4 ↔ 5, Cypress ↔ Playwright, and other test-framework conversions
- **MCP server** for AI assistants (Claude Code, Cursor)
- **Three diagnostic surfaces** for the same artifact — CI status check + JUnit + GitHub annotations, CLI, and an MCP read-only server

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

See [example outputs](docs/examples/) for detailed report samples.

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
- **Not a service.** Terrain analysis is local. No SaaS, no analytics, no account required. Reports stay where you produce them. (Note: `npm install -g mapterrain` and `brew install` download signed binaries from GitHub Releases as part of installation; analysis itself does not phone home.)

If you're evaluating Terrain against another tool and the boundary isn't obvious, please open an issue.

## What 0.2 Is and Isn't

Read this before adopting in CI. Source of truth per-capability is
[`docs/release/feature-status.md`](docs/release/feature-status.md);
this is the summary view.

Capabilities are tiered:

- **Tier 1** — covered by tests, documented behavior, claimed publicly. Floor ≥ 3 on the parity rubric in 0.2.0.
- **Tier 2** — shipping but explicitly experimental; useful but not yet hardened. Floor ≥ 3.
- **Tier 3** — in development, opt-in, no public claim. Wait for promotion.

### By pillar

**Understand** (Tier 1 unless noted):

- `terrain analyze` — snapshot + signals + posture
- `terrain report summary / posture / metrics / insights / explain` — read-side queries
- `terrain compare` — snapshots over time
- AI surface inventory — what AI surfaces exist, where they are, what evals cover them
- `terrain serve` (Tier 2) — local HTTP report; localhost-only, no auth
- `terrain portfolio` (Tier 2, emerging) — multi-repo aggregation; partial in 0.2.0
- `terrain debug *` (Tier 2) — diagnostic drill-downs

**Align** (Tier 1):

- `terrain migrate` / `terrain convert` — framework migration with per-file confidence
- `terrain report select-tests` (Tier 2) — recommended protective test set for a change
- Alignment views in `posture` and `portfolio` — drift between code surface and test surface

**Gate** (Tier 1 unless noted):

- `terrain report pr` — change-scoped PR risk report
- `terrain report impact` — impact selection with reason chains (`--explain-selection`)
- `terrain analyze --fail-on / --timeout / --new-findings-only` — CI gating primitives
- `terrain policy check` — policy enforcement
- Eval artifact ingestion — Promptfoo / DeepEval / Ragas / Great Expectations adapters (plus gauntlet via JSON-compatible ingestion)
- AI risk: **inventory** (Tier 1, reliable)
- AI risk: **hygiene** + **regression** (Tier 2, visible but not gating-critical)
- `terrain ai run` + `terrain ai record` + `terrain ai baseline compare` (Tier 2) — regression-aware AI gate (record a baseline, then compare subsequent runs)

### Anti-goals (0.2.x)

These are explicit non-claims:

- **Terrain does not guarantee safe test skipping.** It provides explainable selection and gating signals. The "see which tests matter — and why" pitch is a clarity claim, not a safe-skip claim.
- **Terrain does not run your tests.** Test runners execute; Terrain reads what they produce.
- **Terrain does not judge model truthfulness.** AI risk detectors surface heuristic structural patterns and ingest eval-framework metadata.
- **Terrain does not promise universal precision floors in 0.2.x.** Coverage broadens in subsequent releases.

## AI Testing in CI

Terrain gives AI components the same CI safety net as regular tests:

- **Surface discovery** — automatically detects prompts, contexts, datasets, tool definitions, RAG pipelines, and eval scenarios in your code
- **Calibrated findings** — `terrain ai findings` emits findings with a confidence score, severity, cohort, and full evidence chain. Per-rule false-positive rate is published per release in the readiness cards
- **Impact-scoped selection** — `terrain ai run --base main` runs only the eval scenarios affected by your change
- **Protection gaps** — `terrain pr` flags changed AI surfaces that have no eval scenario covering them
- **Policy enforcement** — block PRs that modify uncovered AI surfaces, regress accuracy, or trigger safety failures
- **GitHub Action** — drop-in `terrain-ai.yml` workflow template for AI CI gates

The same structural graph that powers test selection for regular code also traces AI surface dependencies, so a change to a prompt template triggers the right eval scenarios automatically.

```bash
terrain ai findings                      # observability posture (≥0.40)
terrain ai findings --posture=gate       # gate posture (≥0.80)
terrain ai findings --json               # CI-consumable output
```

`terrain ai list` (inventory), `terrain ai findings` (calibrated findings via the verdict engine), and the AI catalog detectors (via `terrain analyze`) answer different questions — run all three in CI.

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

> **Node 22 required.** The npm postinstall verifies signed binaries with cosign and uses APIs (`fetch`, top-level await, modern stream primitives) that landed in Node 22. CI images on Node 20 LTS should use the Homebrew or `go install` path. Run `node --version` to check.
>
> **Cosign options.** Install cosign first (`brew install cosign` on macOS/Linux, `scoop install cosign` on Windows). To opt out: set `TERRAIN_INSTALLER_ALLOW_MISSING_COSIGN=1` for checksum-only, or `TERRAIN_INSTALLER_SKIP_VERIFY=1` to skip verification entirely.

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

      # Fail the job (exit 6) if any Critical or High severity signals
      # are present. `--fail-on` is the supported severity gate; distinct
      # exit codes per docs/cli-spec.md. Use `--new-findings-only --baseline`
      # if you're onboarding an established repo with pre-existing debt.
      - run: ./terrain analyze --root . --fail-on=high
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

      # Gate on Critical findings repo-wide (exit 6). For AI-only gating
      # against an eval baseline, use `terrain ai run` — see the
      # walkthrough at docs/examples/gate/ai-eval-ci/.
      - run: terrain analyze --root . --fail-on=critical

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
| `terrain report insights` | What should we fix in our test system? |
| `terrain report impact` | What validations matter for this change? |
| `terrain report explain <target>` | Why did Terrain make this decision? |

### Supporting commands

| Command | Purpose |
|---------|---------|
| `terrain init` | Detect data files and print recommended analyze command |
| `terrain summary` | Executive summary with risk, trends, benchmark readiness |
| `terrain focus` | Prioritized next actions |
| `terrain posture` | Detailed posture breakdown with measurement evidence |
| `terrain portfolio` | Portfolio intelligence: cost, breadth, leverage, redundancy. *(Top-level canonical command, but feature-status: experimental — multi-repo rollups are future work.)* |
| `terrain metrics` | Aggregate metrics scorecard |
| `terrain compare` | Compare two snapshots for trend tracking |
| `terrain select-tests` | Recommend protective test set for a change |
| `terrain pr` | PR/change-scoped analysis |
| `terrain show <type> <id>` | Drill into test, unit, owner, or finding |
| `terrain migration <sub>` | Migration readiness, blockers, or preview |
| `terrain policy check` | Evaluate local policy rules |
| `terrain export benchmark` | Privacy-safe JSON export for benchmarking |
| `terrain serve` | Local HTTP server with HTML report and JSON API |
| `terrain mechanisms list` / `show <name>` | Inspect available detector mechanisms |
| `terrain mcp` | Start the MCP server on stdio for AI assistants |

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

### Environment variables

| Variable | Effect |
|----------|--------|
| `TERRAIN_QUIET=1` | Suppress all stderr chatter (alias migration `[NOTE]`s, legacy deprecation hints). |
| `TERRAIN_ASCII=1` | Force ASCII separators in the discovery report (`terrain` no-args) for non-UTF-8 terminals. |
| `TERRAIN_LEGACY_HINT=1` | Surface canonical-shape suggestions when a legacy command name is invoked. |

## Architecture

Terrain is built around a signal-first architecture. Signals are the core abstraction; snapshots are the canonical serialized artifact; risk surfaces are derived from signals with explainable scoring; dependency graphs model import relationships, fixture fanout, and structural coverage.

See [DESIGN.md](DESIGN.md) for the full architecture and package map and [docs/json-schema.md](docs/json-schema.md) for the JSON output structure.

## Snapshot workflow

Save and compare snapshots to track trends:

```bash
terrain analyze --write-snapshot
# ...later...
terrain analyze --write-snapshot
terrain compare
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
- [Severity](docs/severity-rubric.md) — severity labels and configuration
- [Glossary](docs/glossary.md) — Terrain-specific vocabulary in one page
- [Versioning Policy](docs/versioning.md) — what's a breaking change vs behavior change vs bug fix
- [Compatibility](docs/compatibility.md) — supported OSes, Go versions, frameworks
- [Integrations](docs/integrations/) — Promptfoo / DeepEval / Ragas / gauntlet wiring guides
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
