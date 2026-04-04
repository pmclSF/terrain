# Terrain in 5 Minutes

Terrain analyzes your test system without running any tests. It reads your code, test files, and optional coverage/runtime artifacts to surface risk, quality gaps, and actionable findings.

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

**Key Findings** -- the top 3 issues by severity (duplicates, coverage gaps, high-fanout modules).

**What to do next** -- copy-pasteable commands for the highest-impact next steps.

Below that, you'll see the repository profile, signal breakdown, risk posture, and any structural anomalies. Every finding traces back to specific files and signals.

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
terrain explain <signal-or-test-path>
```

Traces the reasoning behind any finding back to signals, dependency paths, and scoring rules.

## What's next

- [CLI Reference](cli-spec.md) -- all commands and flags
- [Signal Catalog](signal-catalog.md) -- the 22+ signal types Terrain detects
- [Contributing](contributing/adding-a-measurement.md) -- how to extend Terrain
