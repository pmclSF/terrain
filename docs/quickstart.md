# Terrain Quickstart

Five minutes from `npm install` to one actionable insight on your repo.
The walkthrough is structured around the three insights that count toward
the [first-user success gate](../docs/product/vision.md):

1. **PR risk explanation** — a finding tied to a changed file
2. **Coverage gap with explanation** — a named uncovered export with a
   remediation pointer
3. **Test-selection explanation** — chosen tests + reason chains

Terrain operates one layer above your test runners (Jest / pytest / Go
test / Playwright / Promptfoo). It reads what they produce, models the
test system as one thing, and gates against it. No config, no setup,
no test execution required for the walkthrough.

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

## Step 1 — Understand (90 seconds)

The primary workflow's first command. Run this in any repository with
test files:

```bash
terrain analyze
```

You should see a report with the repository profile, signal breakdown,
risk posture, and key findings. **This is your "coverage gap with
explanation" insight**: the report names specific uncovered exports
with remediation pointers ("Add test coverage for 12 uncovered
exported function(s) — see untestedExport signals for specific
functions").

That's the first of the three first-user insights. Two more to go.

## Step 2 — Gate (90 seconds)

The primary workflow's second command. On a feature branch with diff
against main, run:

```bash
terrain report pr --base main
```

This emits a change-scoped PR risk report — what your diff actually
puts at risk, ranked by confidence. **This is your "PR risk
explanation" insight**: every blocking signal is tied to a specific
changed file with a "what / why / what-to-do" line.

Add `--fail-on critical` and the command exits non-zero (code 6) when
any critical-severity finding is present. That's how you wire it
into CI as a gate. See the [CI integration
example](examples/gate/github-action.yml) for the recommended config.

## Step 3 — See which tests matter (90 seconds)

```bash
terrain report impact --base main --explain-selection
```

This is the **test-selection explanation** insight: chosen tests plus
the reason chain for why each was selected and why others were not.

> **Note on safe-skip:** Terrain provides explainable selection. It does
> not assert that any specific test is safe to skip. The "see which
> tests matter — and why" pitch is a clarity claim, not a safe-skip
> claim. Whether to skip the unselected tests is your call, with the
> evidence Terrain hands you.

That's the three first-user insights. From here, the rest of this
guide goes deeper.

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

### AI surface validation

If your codebase has AI components (prompts, RAG pipelines, eval suites), Terrain maps them automatically:

```bash
# See what AI surfaces exist and what's covered
terrain ai list

# Check which eval scenarios a change affects
terrain ai run --base main --dry-run

# Validate AI setup
terrain ai doctor
```

Terrain starts giving AI surfaces CI-visible structure: inventory, impact-scoped eval selection where configured, protection-gap detection, and reviewable risk signals. Suppression workflows and labeled-repo precision floors are 0.3 work — see [`docs/release/0.2-known-gaps.md`](release/0.2-known-gaps.md) for what 0.2 covers and what it doesn't.

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
