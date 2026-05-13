# Terrain

Terrain is the CI gate for AI/ML codebases.

Change a prompt — Terrain knows whether an eval covers it. Add an agent — Terrain knows whether it has a test path. Touch a training pipeline with target leakage — Terrain flags it before the PR lands on main.

It works because Terrain builds one structural graph across your source code, test files, prompt files, eval scenarios, and model artifacts, then fires on the gap edges between them. AI shouldn't be a black box to the codebase. The codebase shouldn't be a black box to AI testing.

Everything runs locally. No SaaS, no account, no analytics.

## Install

```bash
brew install pmclSF/terrain/mapterrain                            # macOS / Linux
npm install -g mapterrain                                         # Node 22+
go install github.com/pmclSF/terrain/cmd/terrain@latest           # Go 1.23+
```

Pre-built binaries are on [GitHub Releases](https://github.com/pmclSF/terrain/releases). Cosign verification and full install detail: [docs/quickstart.md](docs/quickstart.md).

## First report

```bash
cd your-repo
terrain analyze
```

No config, no annotations, no test execution required. Coverage and runtime artifacts get picked up if present, ignored if absent.

## What Terrain catches

The strategic moat is gap detection at the boundary between code and tests, prompts and evals, training and tracking:

- **`promptFileMissingEval`** — a prompt ships in source but no eval covers it
- **`uncoveredAISurface`** — an agent / tool / RAG step has no test path
- **`capabilityValidationGap`** — a declared capability is never validated by any eval
- **`phantomEvalScenario`** — an eval still references a surface that was deleted
- **`targetLeakage` / `missingTrainTestSplit`** — training pipelines with data hygiene gaps

Plus the eval-regression family (`accuracyRegression`, `costRegression`, `hallucinationRate`, `retrievalRegression`) that fires when Promptfoo, DeepEval, or Ragas artifacts are present.

Full detector catalog: [docs/signal-catalog.md](docs/signal-catalog.md).

## The four primary commands

```bash
terrain analyze     # What is the state of our test system?
terrain insights    # What should we fix?
terrain impact      # What tests and evals matter for this change?
terrain explain     # Why did Terrain say that?
```

Every finding carries an evidence chain: signal type, dependency path, confidence, scoring rule. `terrain explain <target>` exposes the full reasoning behind any decision. See [docs/examples/](docs/examples/) for sample output.

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

Onboarding to a repo with existing debt: add `--new-findings-only --baseline=<file>` so prior signals don't fail the first PR.

## Documentation

- [Quickstart](docs/quickstart.md) — first report in 5 minutes
- [Product vision](docs/product/vision.md) — the full narrative
- [Signal catalog](docs/signal-catalog.md) — every detector and what it fires on
- [CLI reference](docs/cli-spec.md) — every command and flag
- [Examples](docs/examples/) — sample analyze / impact / insights / explain output
- [Architecture](DESIGN.md) — unified graph, signal model, package map
- [Feature status](docs/release/feature-status.md) — stable, experimental, planned

[CHANGELOG](CHANGELOG.md) · [CONTRIBUTING](CONTRIBUTING.md) · [SECURITY](SECURITY.md) · License: Apache 2.0
