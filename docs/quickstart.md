# Terrain Quickstart

Understand your test system in 5 minutes. No config, no setup, no test execution required.

Terrain reads your repository — test code, source structure, coverage data, runtime artifacts — and builds a structural model of how your tests relate to your code. From that model it surfaces risk, quality gaps, redundancy, fragile dependencies, and actionable recommendations.

## Install

```bash
# Homebrew
brew install pmclSF/terrain/mapterrain

# npm
npm install -g mapterrain

# Go binary
go install github.com/pmclSF/terrain/cmd/terrain@latest

# From source
git clone https://github.com/pmclSF/terrain.git
cd terrain && go build -o terrain ./cmd/terrain
```

## Your first analysis

Run this in any repository with test files:

```bash
terrain analyze
```

That's it. No configuration, no setup. Terrain auto-detects your test frameworks (Jest, Vitest, Playwright, Cypress, pytest, JUnit, Go testing, and 10 more), builds a structural model, and produces a report.

## Understanding the report

The report starts with the most important information:

**Headline** -- a single sentence summarizing the most surprising finding.

**Key Findings** -- the top issues by severity (duplicates, coverage gaps, high-fanout modules).

**What to do next** -- copy-pasteable commands for the highest-impact next steps.

Below that, you'll see the repository profile, signal breakdown, risk posture, and any structural anomalies. Every finding traces back to specific files and signals.

## What Terrain detects

Out of the box, with no configuration:

- **Frameworks**: Jest, Vitest, Playwright, Cypress, pytest, unittest, Go testing, JUnit, TestNG, Mocha, Jasmine, WebdriverIO, Puppeteer, TestCafe, nose2, and more
- **Languages**: JavaScript, TypeScript, Python, Go, Java
- **Quality signals**: weak assertions, mock-heavy tests, assertion-free tests, orphaned tests, untested exports
- **Health signals**: slow tests, flaky tests, skipped tests, dead tests (requires runtime data)
- **Structural signals**: high-fanout fixtures, duplicate test clusters, coverage gaps
- **Migration signals**: deprecated patterns, framework fragmentation, blocker density
- **AI/eval surfaces**: prompts, contexts, datasets, tool definitions, RAG pipelines, eval scenarios

With optional coverage and runtime data, Terrain also detects coverage breaches, runtime budget violations, and stability clusters.

## Going deeper

### Add coverage data

If your project generates coverage reports, Terrain uses them for precise coverage mapping:

```bash
# Generate coverage (pick your framework)
npx jest --coverage --coverageReporters=lcov
go test -coverprofile=coverage.out ./...
pytest --cov --cov-report=lcov

# Terrain auto-detects common paths, or specify explicitly:
terrain analyze --coverage coverage/lcov.info
```

### Add runtime data

Runtime artifacts (JUnit XML, Jest JSON) unlock health signals -- flaky tests, slow tests, dead tests:

```bash
# Generate runtime artifacts
npx jest --json --outputFile=jest-results.json
go test -json ./... > test-output.json
pytest --junitxml=junit.xml

# Terrain auto-detects common paths, or specify explicitly:
terrain analyze --runtime junit.xml
```

### See what your change affects

```bash
terrain impact --base main
```

This traces your diff through the dependency graph and tells you which tests matter for the change.

### Get prioritized recommendations

```bash
terrain insights
```

A ranked list of findings with effort estimates and suggested actions.

### Understand a specific finding

```bash
# Explain a test file
terrain explain src/auth/login.test.ts

# Explain the overall test selection strategy
terrain explain selection
```

Shows the reasoning behind any finding — which signals fired, what dependency paths are involved, and how scoring rules produced the decision.

### Track trends over time

```bash
# Save a snapshot
terrain analyze --write-snapshot

# Later, compare
terrain compare
```

### Check policy compliance in CI

```bash
terrain policy check --json
# Exit 0 = pass, 2 = violations
```

## The four primary questions

Everything in Terrain maps to one of four questions:

| Command | Question |
|---------|----------|
| `terrain analyze` | What is the state of our test system? |
| `terrain insights` | What should we fix? |
| `terrain impact --base main` | What tests matter for this change? |
| `terrain explain <target>` | Why did Terrain make this decision? |

## Supporting views

| Command | Purpose |
|---------|---------|
| `terrain summary` | Executive summary with risk and trends |
| `terrain focus` | Prioritized next actions |
| `terrain posture` | Measurement evidence by dimension |
| `terrain portfolio` | Cost, breadth, leverage, redundancy |
| `terrain metrics` | Aggregate metrics scorecard |
| `terrain select-tests` | Protective test set for CI |
| `terrain pr --base main` | PR-scoped analysis |
| `terrain show test <path>` | Drill into a specific test file |

All commands support `--json` for machine-readable output and `--root PATH` to target a specific repository.

## What's next

- [CLI Reference](cli-spec.md) -- all commands and flags
- [Signal Catalog](signal-catalog.md) -- the signal types Terrain detects
- [Example Reports](examples/) -- sample output for each command
- [Contributing](contributing/adding-a-measurement.md) -- how to extend Terrain
