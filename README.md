# Terrain

**Pre-flight checks for AI/ML systems and the tests around them. Runs locally. No API key required.**

Terrain is a static analyzer that treats tests, evals, prompts, and schemas as one dependency graph — and catches the drift that crosses their boundaries before it reaches production.

## Install

```bash
# macOS / Linux
brew install pmclSF/terrain/mapterrain

# npm (Node 22+ required)
npm install -g mapterrain

# Go
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

Pre-built binaries for macOS / Linux / Windows are on the [releases page](https://github.com/pmclSF/terrain/releases). Each release is signed with Sigstore + cosign; see [SECURITY-DATA-HANDLING.md](SECURITY-DATA-HANDLING.md) for verification details.

## Get started

```bash
cd your-repo
terrain analyze         # What's the state of our AI + test system?
terrain report pr       # What does this change put at risk?
```

Works on repos in Python, TypeScript/JavaScript, Go, Java, and Ruby. No config required. Optional artifacts (coverage data, runtime test results, eval outputs) sharpen findings when present; analysis degrades gracefully when they're absent.

## What it catches

Terrain models the AI surface alongside the test surface and looks for drift across them:

- **Prompt-schema drift** — prompts that reference fields renamed in a schema living in a different language
- **Hardcoded API keys** — provider-shaped secrets in source (OpenAI, Anthropic, AWS, GCP, etc.)
- **Eval coverage gaps** — AI surfaces (prompts, agents, RAG pipelines) with no scenario covering them
- **Model deprecations** — deprecated model IDs lingering in production paths
- **Cross-language edges** — TS/JS ↔ Python/Go/Java via OpenAPI, tRPC, gRPC, GraphQL, HTTP routes
- **Framework-migration blockers** — Jest ↔ Vitest, JUnit 4 ↔ 5, Cypress ↔ Playwright
- **Untested exports + weak assertions** — public API surfaces with no covering test; assertions that pass on too much
- **Fixture fanout + duplicate clusters + skip debt** — structural problems that erode CI signal

Each finding carries a stable rule ID, severity, confidence, evidence, and documented remediation. Run `terrain explain <rule-id>` for the long form.

## What it looks like

```
Terrain — Test Suite Analysis
============================================================

  conftest.py fixture fans out to 3,100 tests — any change retriggers the frame/ suite.

Key Findings
------------------------------------------------------------
  1. [HIGH] 23 source files (18%) have low structural coverage
  2. [HIGH] 8 duplicate test clusters with 0.91+ similarity
  3. [MEDIUM] 34 xfail markers older than 180 days

Risk Posture
------------------------------------------------------------
  health:              MODERATE
  coverage_depth:      ELEVATED
  coverage_diversity:  STRONG
  structural_risk:     STRONG
```

> Representative output from a large pandas-style repository. Format and labels are stable across runs; the specific numbers vary by repo. Full sample reports for `analyze`, `insights`, `impact`, and `explain` are in [docs/examples/](docs/examples/).

## Workflow

Four commands cover most adopter use:

| Command | Question |
|---------|----------|
| `terrain analyze` | What is the state of our test system? |
| `terrain report insights` | What should we fix? |
| `terrain report impact` | What validations matter for this change? |
| `terrain report explain <target>` | Why did Terrain make this decision? |

The bare forms (`terrain insights`, `terrain impact`, `terrain explain`) are preserved as aliases. The full command surface — including AI / eval verbs, framework conversion, debug drill-downs, and the slash-receiver — is in [docs/cli-spec.md](docs/cli-spec.md).

## CI integration

```yaml
# .github/workflows/terrain.yml
name: terrain
on: pull_request

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with: { fetch-depth: 0 }
      - uses: actions/setup-node@v6
        with: { node-version: '22.x' }
      - run: npm install -g mapterrain
      - run: |
          terrain test \
            --junit terrain-results.xml \
            --summary "$GITHUB_STEP_SUMMARY"
```

`terrain test` is the CI-mode wrapper. The JUnit XML lets your CI render Terrain findings as test cases; setting `--summary "$GITHUB_STEP_SUMMARY"` makes them appear on the workflow run page automatically.

For a blocking gate, add `--fail-on=high` (or `--fail-on=critical` for the strictest threshold). For onboarding a repo with existing debt, pair with `--new-findings-only --baseline <path>` so only regressions block the build. To restrict the gate to AI-related changes, add a `paths:` filter on prompt / schema / Python / TS file globs (see [docs/examples/gate/](docs/examples/gate/) for the full templates).

## What it is not

The test-tooling space has a lot of overlapping vendors and the boundaries matter:

- **Not a test runner.** Terrain reads what `pytest` / `jest` / `go test` / `playwright` produce. It doesn't execute tests.
- **Not a coverage tool.** Terrain ingests coverage reports if you have them; it doesn't instrument code or compute coverage itself.
- **Not a static analyzer for application code.** Sonar / Semgrep / CodeQL stay better-positioned for source-side bug-finding.
- **Not an LLM eval framework.** Terrain understands AI surfaces and ingests the artifacts Promptfoo / DeepEval / Ragas produce. It doesn't run the evals.
- **Not a developer-productivity dashboard.** Terrain measures the test system, not the people writing tests. No leaderboards. No per-developer metrics. Ownership data routes findings, never scores people.
- **Not a service.** Analysis is local. No SaaS. No telemetry. No account. Verifiable with `terrain --print-network`. (npm and brew install paths download signed binaries from GitHub Releases; analysis itself never phones home.)

## AI-aware testing

The same dependency graph that powers test selection for application code also traces AI surface edges, so a prompt-template change triggers the right eval scenarios automatically. `terrain ai run --base main` runs only the evals affected by your change; `terrain report pr` flags changed AI surfaces with no covering eval; `terrain ai findings` emits AI eval-gap findings with evidence chains.

Two generators help adopters harden a prompt before deploying it:

```bash
terrain inject --prompt prompts/main.md       # generate jailbreak-shaped test inputs
terrain scaffold --schema schemas/input.json  # generate boundary-case mutation tests
```

Both emit runnable pytest / vitest scaffolds you drop into your test tree. Terrain never calls the model; the assertion is yours.

## MCP integration

`terrain mcp` starts an [MCP](https://modelcontextprotocol.io) server on stdio for AI coding assistants (Claude Code, Cursor, etc.). It reads the last `terrain analyze` snapshot from `.terrain/findings.json` and exposes findings, surfaces, and baselines as tools the assistant can query.

## Plugins

Third-party detectors ship as plugins with a YAML manifest; validate one with `terrain plugins manifest <path>`. The manifest contract is stable today (see [examples/plugins/example-manifest.yaml](examples/plugins/example-manifest.yaml)); the runtime that loads and executes plugins is reserved for a future release.

## Documentation

**Get started**

- [Quickstart](docs/quickstart.md) — first report in 5 minutes
- [CLI specification](docs/cli-spec.md) — full command + flag reference
- [Example reports](docs/examples/) — analyze / impact / insights / explain samples

**Reference**

- [Design](DESIGN.md) — architecture, package map, signal pipeline
- [Severity rubric](docs/severity-rubric.md) — severity labels and configuration
- [Compatibility](docs/compatibility.md) — supported OSes, Go versions, frameworks, schemas
- [Glossary](docs/glossary.md) — Terrain-specific vocabulary
- [Versioning](docs/versioning.md) — what counts as a breaking change

**Project**

- [CHANGELOG](CHANGELOG.md) — release history
- [Security](SECURITY.md) — supported versions + vulnerability disclosure
- [Contributing](CONTRIBUTING.md) — how to build, test, and extend Terrain

## License

Apache License 2.0 — see [LICENSE](LICENSE).
