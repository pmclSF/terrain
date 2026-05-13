# Terrain

Terrain is a CI gate for AI/ML codebases. It builds one structural graph spanning your source code, test files, prompt files, eval scenarios, and model artifacts — then fires on the gap edges between them. Default detectors flag the AI and ML hygiene gaps that no other tool sees: prompts shipping without eval coverage, AI surfaces with no test path, declared capabilities never validated, training pipelines with leakage risk.

The thesis: AI shouldn't be a black box to the codebase, and the codebase shouldn't be a black box to AI testing. Static analyzers see source. Test runners see tests. Eval frameworks see prompts. Terrain sees all four surfaces at once and gates pull requests against the unified picture.

Generic test-quality detectors are still in the binary, but they're demoted to Low unless they participate in a boundary signal. Terrain doesn't replace Ruff or ESLint on the source side, or Jest / pytest / go test on the runner side. It operates one layer above — reading what the runners produce, modeling the system as one thing, and gating against it.

Everything runs locally. No SaaS, no account, no analytics. Reports stay on the machine that produced them.

## Install

```bash
# Homebrew (macOS / Linux)
brew install pmclSF/terrain/mapterrain

# npm (Node 22+ required for signed-binary verification)
npm install -g mapterrain

# Go install (Go 1.23+)
go install github.com/pmclSF/terrain/cmd/terrain@latest
```

Pre-built binaries for macOS / Linux / Windows are on [GitHub Releases](https://github.com/pmclSF/terrain/releases). Detailed install paths and cosign verification are in [docs/quickstart.md](docs/quickstart.md).

## First report in 30 seconds

```bash
cd your-repo
terrain analyze
```

No config, no annotations, no test execution required. Coverage and runtime artifacts are picked up automatically if present, ignored if absent. Stronger findings (runtime health, eval regression, policy enforcement) unlock when those artifacts exist.

## What Terrain detects

The AI/ML-side detectors that ship in 0.2:

| Detector | Fires when |
|---|---|
| `promptFileMissingEval` | A prompt file has no eval scenario covering it |
| `uncoveredAISurface` | An AI surface (agent / tool / RAG step) has no covering test |
| `phantomEvalScenario` | An eval references a surface that no longer exists |
| `untestedPromptFlow` | A prompt flow connects surfaces with no end-to-end coverage |
| `capabilityValidationGap` | A declared capability is never validated by any eval |
| `aiSafetyEvalMissing` | A safety-tagged AI surface ships without a safety eval |
| `aiPromptInjectionRisk` | Source code structurally permits prompt injection |
| `aiHardcodedAPIKey` | API keys appear in source (heuristic) |
| `aiToolWithoutSandbox` | An agent tool definition lacks sandboxing markers |
| `aiModelDeprecationRisk` | A deprecated model string is still referenced |
| `targetLeakage` / `missingTrainTestSplit` / `dataLeakageSuspected` | ML training pipelines with dataset hygiene gaps |

Plus the eval-regression family (`accuracyRegression`, `costRegression`, `hallucinationRate`, `retrievalRegression`, `latencyRegression`, ...) that fires when Promptfoo / DeepEval / Ragas artifacts are present.

[Full detector catalog →](docs/signal-catalog.md)

## The four primary commands

```bash
terrain analyze     # What is the state of our test system?
terrain insights    # What should we fix?
terrain impact      # What tests and evals matter for this change?
terrain explain     # Why did Terrain say that?
```

Every finding carries an evidence chain: signal type, confidence, dependency path, scoring rule. `terrain explain <target>` exposes the full reasoning behind any decision. See [docs/examples/](docs/examples/) for sample output.

Other useful commands: `terrain pr` (PR-scoped report), `terrain ai list` (AI surface inventory), `terrain ai run` (impact-scoped eval execution), `terrain policy check` (local policy enforcement). Full reference: [docs/cli-spec.md](docs/cli-spec.md).

## CI integration

```yaml
# .github/workflows/terrain.yml
name: terrain
on: pull_request

jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with: { fetch-depth: 0 }
      - uses: actions/setup-node@v6
        with: { node-version: '22.x' }
      - run: npm install -g mapterrain
      - run: terrain analyze --root . --fail-on=high
```

`--fail-on=high` exits non-zero (code 6) on Critical or High findings. For onboarding to an established repo, add `--new-findings-only --baseline=<file>` so existing debt doesn't fail the first PR. AI-specific gating and Strict / Minimal templates: [docs/examples/gate/](docs/examples/gate/).

## What Terrain doesn't do

- **Doesn't run your tests.** Test runners execute; Terrain reads their artifacts.
- **Doesn't instrument coverage.** Bring `c8` / `istanbul` / `coverage.py` / `gcov`; Terrain turns coverage into structural insight.
- **Doesn't compete with Ruff / ESLint / Semgrep / Sonar.** Source-side bug-finding stays with them.
- **Doesn't execute evals.** Promptfoo / DeepEval / Ragas run them; Terrain ingests their output.
- **Doesn't profile developers.** Test-system health only, never per-person metrics or leaderboards.
- **Doesn't phone home.** Analysis is local; installation downloads signed binaries from GitHub Releases.

## Snapshots and trend tracking

```bash
terrain analyze --write-snapshot   # Save snapshot to .terrain/snapshots/
terrain compare                    # Compare the two most recent
terrain summary                    # Executive summary with trend highlights
```

## Policy

Define local policy rules in `.terrain/policy.yaml`:

```yaml
rules:
  disallow_skipped_tests: true
  max_weak_assertions: 10
  max_mock_heavy_tests: 5
```

Then: `terrain policy check` (exit 0 pass, 2 violations, 1 error).

## What's stable in 0.2 vs. what's planned

Tier 1 (covered by tests, documented, claimed): `terrain analyze`, `terrain insights`, `terrain impact`, `terrain explain`, `terrain pr`, `terrain policy check`, AI surface inventory, eval-artifact ingestion, the gate-tier detectors.

Tier 2 (shipping but explicitly experimental): `terrain serve`, `terrain portfolio`, the AI hygiene + regression detectors, regression-aware `terrain ai run` + `record` + `baseline compare`.

Tier 3 (in development): per-detector precision benchmarking against a labeled corpus, AST-grade prompt-injection taint analysis, suppression lifecycle, sandboxed eval execution.

Full per-capability status: [docs/release/feature-status.md](docs/release/feature-status.md).

## Architecture

```
Repository scan → Signal detection → Risk modeling → Reporting
   test files       framework-aware    explainable      human-readable
   source files     pattern detectors  risk scoring     + JSON output
   prompt files     boundary edges     with evidence
   eval scenarios   gap detection      chains
```

- **Signals** are the core abstraction — every finding is a structured signal with type, severity, confidence, evidence, and location.
- **Snapshots** (`TestSuiteSnapshot`) are the canonical serialized artifact — the full structural model at a point in time.
- **Unified graph** spans source files, test files, prompt files, eval scenarios, model artifacts, and the typed edges between them.

[DESIGN.md](DESIGN.md) has the package map. [docs/architecture/](docs/architecture/) has the technical design documents.

## Documentation

- [Quickstart](docs/quickstart.md) — first report in 5 minutes
- [Product vision](docs/product/vision.md) — the full narrative
- [Signal catalog](docs/signal-catalog.md) — every detector, what it fires on
- [CLI reference](docs/cli-spec.md) — every command and flag
- [JSON schema](docs/json-schema.md) — for tooling that reads Terrain output
- [Examples](docs/examples/) — sample analyze / impact / insights / explain reports
- [Feature status](docs/release/feature-status.md) — stable / experimental / planned
- [Compatibility](docs/compatibility.md) — supported OSes, Go versions, frameworks
- [CHANGELOG](CHANGELOG.md), [CONTRIBUTING](CONTRIBUTING.md), [SECURITY](SECURITY.md)

## Development

```bash
go build -o terrain ./cmd/terrain
go test ./cmd/... ./internal/...
make release-verify
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
