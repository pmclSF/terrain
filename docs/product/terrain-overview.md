# Terrain — Product Overview

> **Terrain is a test system intelligence platform. It maps your test terrain.**

Terrain reads your repository — test code, source structure, coverage data, runtime artifacts, ownership files, and local policy — and builds a structural model of how your tests relate to your code. From that model it surfaces risk, quality gaps, redundancy, fragile dependencies, and migration readiness, all without running a single test.

## Design Principles

**Inference-first.** Terrain infers structure from what already exists in your repo — import graphs, file naming, coverage artifacts, runtime results. No configuration required to get started. `terrain init` detects available data sources; `terrain analyze` produces a complete assessment.

**Explainability-first.** Every finding carries an evidence chain. `terrain explain` traces any decision back to the signals, dependency paths, and scoring rules that produced it. No black boxes, no magic numbers.

**Signal-first.** The core abstraction is the signal — a typed, located, severity-scored, confidence-weighted finding with an explanation and suggested action. All 22 signal types flow through the same registry, making the system extensible and auditable.

**Evidence-graded.** Terrain explicitly states the strength of its evidence (strong, partial, weak, none) and adjusts confidence accordingly. It tells you what it knows, what it inferred, and what it cannot see.

## The Four Canonical Workflows

Everything in Terrain is organized around four questions:

| Command | Question | Decision Moment |
|---------|----------|-----------------|
| `terrain analyze` | What is the state of our test system? | Sprint planning, onboarding, audits |
| `terrain insights` | What should we fix or improve? | Backlog grooming, tech debt triage |
| `terrain impact` | What validations matter for this change? | PR review, release readiness |
| `terrain explain` | Why did Terrain say this? | Debugging findings, building trust |

These four commands are the product front door. All other commands — `summary`, `posture`, `focus`, `portfolio`, `metrics`, `compare`, `migration`, `policy`, `select-tests`, `pr`, `show`, `export` — are supporting views that answer follow-up questions from the same underlying model.

## What Terrain Analyzes

**Languages:** JavaScript/TypeScript, Go, Python, Java.

**Frameworks:** 17 test frameworks across 4 languages — Jest, Vitest, Mocha, Jasmine, Playwright, Cypress, Puppeteer, WebdriverIO, TestCafe, Node Test, pytest, unittest, nose2, go-testing, JUnit 4, JUnit 5, TestNG.

**Data sources:**
- Source code and test code (static analysis — always available)
- Coverage artifacts (LCOV, Istanbul JSON — optional, enriches analysis)
- Runtime artifacts (JUnit XML, Jest/Vitest JSON — optional, enables health signals)
- CODEOWNERS files (ownership attribution)
- `.terrain/terrain.yaml` (manual coverage overlay, external QA tool mapping)
- `.terrain/policy.yaml` (governance rules)
- Git history (change impact analysis)

## Signal Categories

| Category | Signals | What They Detect |
|----------|---------|------------------|
| **Health** | slowTest, flakyTest, skippedTest, deadTest, unstableSuite | Runtime health and operational risk |
| **Quality** | untestedExport, weakAssertion, mockHeavyTest, testsOnlyMocks, snapshotHeavyTest, coverageBlindSpot | Structural quality and coverage depth |
| **Migration** | frameworkMigration, migrationBlocker, deprecatedTestPattern, dynamicTestGeneration, customMatcherRisk | Migration readiness and blockers |
| **Governance** | policyViolation, legacyFrameworkUsage, skippedTestsInCI, runtimeBudgetExceeded | Policy compliance and organizational standards |

Each signal includes: type, category, severity (Low/Medium/High), confidence (0.0–1.0), location (file + symbol), explanation, suggested action, and metadata.

## Measurement Model

Terrain computes posture across five dimensions:

1. **Health** — Skip burden, flaky rate, dead test share, runtime stability
2. **Coverage Depth** — Untested export share, weak assertion share, threshold compliance
3. **Coverage Diversity** — Mock concentration, framework fragmentation, E2E-only coverage, unit test reach
4. **Structural Risk** — Fanout burden (high-dependency modules), duplicate test clusters
5. **Operational Risk** — Policy violations, governance compliance

These dimensions roll up into a posture band (strong, moderate, weak, elevated, critical) with explicit evidence grading for each dimension. When insufficient data is available, a dimension may report as unknown.

## Who Uses Terrain

Terrain serves six primary personas, each asking different questions of the same underlying graph:

| Persona | Primary Concern | Key Workflow |
|---------|----------------|--------------|
| **Frontend Engineer** | Component coverage, snapshot debt, framework fragmentation | `impact` on component refactors; `insights` for snapshot-heavy files |
| **Backend Engineer** | Transitive dependency risk, service boundary coverage | `impact` for blast radius; `debug fanout` for high-risk modules |
| **Mobile / Device Engineer** | Skip debt from platform dependencies, CI cost | `analyze` for skip burden; `insights` for platform-conditional gaps |
| **QA / SDET** | Test health trends, policy compliance, manual coverage overlay | `policy check`; `posture`; `compare` for trend tracking |
| **SRE / Platform Engineer** | CI duration, flaky tests, test selection | `pr` in CI; `select-tests` for fast feedback; `metrics` for dashboards |
| **AI / ML Engineer** | Eval suite coverage, parametrized test auditing | `analyze` for eval audit; `insights` for redundant evals |

See [Persona Journeys](../architecture/17-persona-journeys.md) and [Feature Matrix](feature-matrix.md) for detailed capability mapping.

## CI Integration

Terrain integrates into CI as a CLI step. The primary integration point is the GitHub Actions composite action at `.github/actions/terrain-impact/`, which:

1. Builds the Go binary
2. Runs impact analysis against the PR diff
3. Parses results (test count, risk level, coverage confidence)
4. Posts/updates a PR comment with the impact summary
5. Uploads the impact artifact for downstream consumption
6. Writes a job summary

Test selection uses a 5-level fallback ladder to ensure safety: exact match → inferred match → transitive match → heuristic match → full suite.

## Architecture

```
cmd/terrain/         CLI entry point and command routing
internal/
├── engine/          Pipeline orchestration, detector registry
├── analysis/        Repository scanning, content analysis, language detection
├── signals/         Signal type registry and construction
├── depgraph/        Dependency graph: coverage, fanout, duplicates
├── scoring/         Risk scoring engine
├── governance/      Policy evaluation
├── quality/         Quality detectors (assertions, mocks, snapshots, coverage)
├── health/          Health detectors (slow, flaky, skipped, dead, unstable)
├── migration/       Migration readiness and blocker detection
├── impact/          Change impact analysis and test selection
├── metrics/         Aggregate metrics computation
├── summary/         Executive summary generation
├── ownership/       CODEOWNERS parsing and attribution
├── reporting/       Output formatting (text, JSON, markdown)
└── testcase/        Test case extraction and identity
```

The pipeline runs in a fixed order: scan → analyze → detect signals → build dependency graph → score → report. Each detector registers with the engine and receives the accumulated state from prior stages.

## Legacy Converter Engine

The JavaScript converter engine (`src/`, `bin/terrain.js`) provides multi-framework test conversion across 16 frameworks and 25 conversion directions. It remains functional and is accessible via `node bin/terrain.js` (the npm package entry point). The converter is not currently wired into the Go CLI binary. This engine predates the Go analysis engine and serves as the migration acquisition wedge — the pain of framework migration is what brings teams to Terrain, and the analysis engine turns that pain into broader test intelligence.

## Current State

**Analysis Engine:**
- **22 signal types** across 4 categories (health, quality, migration, governance)
- **17 test frameworks** detected across 4 languages (JS/TS, Go, Python, Java)
- **7 code surface kinds** — function, method, handler, route, class, prompt, dataset
- **5 posture dimensions** with 18 measurements and evidence grading
- **6 policy rule types** with size-aware thresholds
- **14 edge case detectors** with adaptive confidence adjustment

**Graph & Reasoning:**
- **20 node types** across 6 families (system, validation, behavior, environment, execution, governance)
- **15 edge types** with confidence scoring and evidence types
- **5 reasoning pipelines** — impact, coverage, redundancy, stability, environment

**CLI:**
- **4 canonical commands** — analyze, impact, insights, explain
- **13 supporting commands** — init, summary, focus, posture, portfolio, metrics, compare, migration, policy, select-tests, pr, show, export benchmark
- **6 AI commands** — ai list, ai doctor, ai run, ai replay, ai record, ai baseline
- **5 debug commands** — debug graph, coverage, fanout, duplicates, depgraph

**AI Validation:**
- **Scenario model** — first-class validation targets alongside tests
- **Prompt and dataset inference** — naming convention detection in JS/TS and Python
- **Scenario impact detection** — changed prompt/dataset surfaces mapped to impacted scenarios
- **Gauntlet integration** — artifact ingestion via `--gauntlet` flag

**Infrastructure:**
- **CI integration** via GitHub Actions composite action with PR comments and artifact upload
- **Golden/snapshot tests** validating all 4 canonical journeys
- **20 benchmark repositories** — 10 real-world (express, flask, vue, jest, playwright, storybook, next.js, fastify, gauntlet, terrain) + 10 fixture repos
- **Benchmark smoke tests** in CI validating 4 canonical commands across 3 fixture types
