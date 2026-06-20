# Terrain

**Open-source CI pre-flight layer for AI/ML systems and the tests around them. Runs locally. No API key required.**

Rename `customer_name` to `full_name`, and a prompt in another part of the repo may still ask the model for `customer_name`. Change a retriever, and the eval that would catch the quality drop may never run. Terrain catches those gaps in the PR, before the change merges.

Terrain does that by connecting source code, tests, prompts, schemas, eval outputs, runtime artifacts, and coverage reports into one CI-ready protection graph. It turns that graph into ranked findings, explainable impact, JUnit test cases, GitHub annotations, SARIF for security findings, MCP context for AI assistants, and portfolio rollups across repos.

## Why install Terrain?

Most teams already have tests, coverage, and AI evals. The hard part is knowing whether those signals still protect the change in front of you. A developer can see that unit tests passed or that an eval failed, but still not know whether a PR touched an AI/ML surface, which validations should run, what coverage is missing, or why a downstream regression is connected to their diff.

Terrain gives every PR a pre-merge protection map. In one local command it answers:

- What is weak or missing in the current test system?
- What does this PR put at risk?
- Which tests or evals matter for this change?
- Why did the gate fail, and what should be fixed first?

That makes day-to-day work less manual:

- Instead of tracing code -> API -> prompt -> eval by hand, run `terrain report pr` and see what the change affects.
- Instead of rerunning every eval because no one knows which one matters, run the validations Terrain selects for the diff.
- Instead of reading a failed eval in isolation, use the cause path to see which code, schema, prompt, or retrieval change led there.
- Instead of letting weak assertions, skipped tests, and uncovered exports accumulate quietly, surface them as concrete findings before release.

The result is faster review, fewer broad reruns, clearer fixes, and fewer regressions that only make sense after someone reconstructs the system by hand.

Bring your own stack. Terrain reads what teams already use: `pytest`, `jest`, `go test`, Playwright, Promptfoo, DeepEval, Ragas, Great Expectations, Gauntlet-style eval-result JSON, JUnit, LCOV, Istanbul, and repository metadata. It does not replace those tools; it unifies their evidence into one local, deterministic CI gate.

Project status: Terrain is pre-1.0 and actively developed. In 0.3.0, the stable path is the CLI, local/CI artifact contract, GitHub Actions flow, AI/eval artifact ingestion, and portfolio aggregation. The VS Code extension is alpha; marketplace listings, full LSP integration, the plugin runtime, and some preview rules are future work.

## Install

```bash
# macOS / Linux
brew install pmclSF/terrain/mapterrain

# npm (Node 22+ required; macOS/Linux amd64+arm64, Windows amd64)
npm install -g mapterrain

# Go
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

Pre-built archives are available for macOS and Linux on amd64/arm64, plus Windows on amd64, from the [releases page](https://github.com/pmclSF/terrain/releases). Each release is signed with Sigstore + cosign; see [SECURITY-DATA-HANDLING.md](SECURITY-DATA-HANDLING.md) for verification details.

Package names vary by distribution, but the installed CLI is always `terrain`. The npm package is `mapterrain`; the Homebrew formula is `pmclSF/terrain/mapterrain`.

## Get started

```bash
cd your-repo
terrain analyze         # What's the state of our AI + test system?
terrain report pr       # What does this change put at risk?
```

Source analysis covers Python, TypeScript/JavaScript, Go, and Java in 0.3.0. Ruby source is not analyzed in 0.3.0, but Ruby/RSpec and other ecosystems can still contribute dependency, runtime, coverage, and eval artifacts, so mixed-language repos get useful signal. No config required; optional artifacts sharpen findings when present.

## What it catches

Terrain models the AI surface alongside the test surface and looks for drift across them:

- **Prompt-schema drift** — prompts that reference fields renamed in a schema living in a different language
- **Hardcoded API keys** — provider-shaped secrets in source (OpenAI, Anthropic, AWS, GCP, etc.)
- **Eval coverage gaps** — AI surfaces (prompts, agents, RAG pipelines) with no scenario covering them
- **Model deprecations** — deprecated model IDs lingering in production paths
- **Cross-language edges** — TS/JS ↔ Python/Go/Java via OpenAPI, tRPC, gRPC, GraphQL, HTTP routes
- **Framework-migration blockers** — Jest ↔ Vitest, JUnit 4 ↔ 5, Cypress ↔ Playwright
- **Portfolio drift across repos** — manifest-backed `terrain portfolio --from` rollups show framework-of-record drift across a polyrepo
- **Untested exports + weak assertions** — public API surfaces with no covering test; assertions that pass on too much
- **Fixture fanout + duplicate clusters + skip debt** — structural problems that erode CI signal

Each finding carries a stable rule ID, severity, confidence, evidence, and documented remediation. Run `terrain explain <rule-id>` for the long form.

## What it looks like

```
Terrain · Test Suite Analysis
────────────────────────────────────────────────────────────

  conftest.py fixture fans out to 3,100 tests — any change retriggers the frame/ suite.

Key Findings
────────────────────────────────────────────────────────────
  1. [HIGH] 23 exported code units have no linked tests
  2. [MED]  12 test files have weak assertion density
  3. [LOW]  7 skipped-test patterns need review

Risk Posture
────────────────────────────────────────────────────────────
  health:                  Moderate
  coverage_depth:          Elevated
  coverage_diversity:      Strong
  structural_risk:         Strong
  Signals:                 65 (8 high, 34 medium, 23 low)
```

> Representative output from a large pandas-style repository. Format and labels are stable across runs; the specific numbers vary by repo. Full sample reports for `analyze`, `insights`, `impact`, and `explain` are in [docs/examples/](docs/examples/).

## Workflow

| Command | Question |
|---------|----------|
| `terrain analyze` | What is the state of our test system? |
| `terrain report pr` | What does this change put at risk? |
| `terrain report insights` | What should we fix? |
| `terrain report impact` | What validations matter for this change? |
| `terrain report explain <target>` | Why did Terrain make this decision? |

The bare forms (`terrain insights`, `terrain impact`, `terrain explain`) work as aliases. AI / eval verbs, framework conversion, debug drill-downs, and the slash-receiver round out the surface — full reference in [docs/cli-spec.md](docs/cli-spec.md).

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

## Boundaries

Terrain is intentionally a CI/local pre-flight layer. It composes with the tools teams already trust instead of replacing them:

- It reads what `pytest`, `jest`, `go test`, Playwright, Promptfoo, DeepEval, Ragas, Great Expectations, and Gauntlet-style eval-result artifacts produce. The AI namespace can execute supported eval-framework commands for `terrain ai run`, but Terrain does not replace those frameworks.
- It ingests coverage reports when you have them; it does not instrument code.
- It complements Semgrep / CodeQL / Sonar rather than replacing application-code bug finding.
- It routes ownership data to findings; it never produces leaderboards or per-developer scores.
- It runs locally with zero outbound network calls during analysis — verifiable with `terrain --print-network`. The install paths download signed binaries from GitHub Releases; Terrain itself does not upload analysis results.
- CI artifacts and MCP responses can include repo paths, line numbers, diagnostic text, and sometimes source excerpts. Treat the CI platform or AI assistant you connect as part of the trust boundary.

Under the hood, Terrain combines source analysis, dependency-graph construction, artifact ingestion, and deterministic rule evaluation. The product is the CI gate and protection graph; static analysis is one mechanism inside it.

## AI-aware testing

The same dependency graph that powers test selection for application code also traces AI surface edges. When Terrain can map a prompt-template, schema, or eval-artifact change to declared or inferred scenarios, it selects the impacted evals automatically.

- `terrain ai run --base main` — run only the evals your change affects
- `terrain report pr` — flag changed AI surfaces with no covering eval
- `terrain ai findings` — AI eval-gap findings with evidence chains

Two generators help adopters harden a prompt before deploying it:

```bash
terrain inject --prompt prompts/main.md       # generate jailbreak-shaped test inputs
terrain scaffold --schema schemas/input.json  # generate boundary-case mutation tests
```

Both emit runnable pytest / vitest scaffolds you drop into your test tree. Terrain never calls the model; the assertion is yours.

## MCP integration

`terrain mcp` exposes the last analyze run to AI coding assistants (Claude Code, Cursor, others) over the [Model Context Protocol](https://modelcontextprotocol.io). The assistant can query findings, drill into surfaces, and read baselines without you copy-pasting JSON — so "why is this PR failing Terrain?" gets a useful answer in the IDE instead of a context-switch to the terminal.

The MCP server is local and read-only. The assistant client decides what context, if any, is sent to its model provider.

## Plugins

Third-party detectors ship as YAML manifests; `terrain plugins manifest <path>` validates one against the stable schema. The plugin runtime — the loader that executes registered detectors — is reserved for a future release. Adopters can author and publish manifests now and they'll be loadable when the runtime ships. See [`examples/plugins/example-manifest.yaml`](examples/plugins/example-manifest.yaml).

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
