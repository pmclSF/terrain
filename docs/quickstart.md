# Terrain Quickstart

Five minutes from install to three actionable insights on your repo. No config, no setup, no test execution required.

## Install

```bash
brew install pmclSF/terrain/mapterrain        # Homebrew (macOS / Linux)
npm install -g mapterrain                     # npm (Node 22+)
go install github.com/pmclSF/terrain/cmd/terrain@latest  # Go
```

Or download a signed binary from the [releases page](https://github.com/pmclSF/terrain/releases).

## 1. Understand your test system

```bash
cd your-repo
terrain analyze
```

You'll see a one-line headline (the most surprising finding), a Key Findings block ranked by severity, repository profile, and risk posture. Every finding names specific files and signals — no vague handwaving.

## 2. See what your change puts at risk

On a feature branch with a diff against `main`:

```bash
terrain report pr --base main
```

A change-scoped risk report — what your diff actually affects, ranked by confidence, each line tied to a specific file. Add `--fail-on=high` to turn it into a CI gate (exit code 6 on blocking findings).

## 3. See which tests matter for the change

```bash
terrain report impact --base main --explain-selection
```

The selected tests plus the reason chain for each. `--explain-selection` shows why a test made the cut and why others didn't — clarity, not safe-skip advice.

That's the canonical adopter loop. Everything below is depth.

---

## Adding optional inputs

Terrain reads what your tools already produce. Each integration is one flag:

```bash
# Coverage data → precise structural coverage mapping
pytest --cov --cov-report=lcov
terrain analyze --coverage coverage/lcov.info

# Runtime artifacts → flaky / slow / dead test detection
pytest --junitxml=junit.xml
terrain analyze --runtime junit.xml

# Snapshot for trend tracking
terrain analyze --write-snapshot
terrain compare        # later
```

## Wire into CI

`terrain test` is the CI-mode wrapper. It writes JUnit XML (GitHub's test reporter shows Terrain findings as test cases) and a markdown step-summary (visible on the workflow run page):

```yaml
- name: Terrain pre-flight
  run: |
    terrain test \
      --junit terrain-results.xml \
      --summary "$GITHUB_STEP_SUMMARY"
```

The `--summary` value is the file Terrain writes; `$GITHUB_STEP_SUMMARY` is the file GitHub Actions reads. Set them equal and findings appear on the run page automatically. Full templates in [`docs/examples/gate/`](examples/gate/).

## AI surfaces

If your repo has prompts, RAG pipelines, or eval suites, Terrain maps them automatically:

```bash
terrain ai list                          # inventory
terrain ai run --base main --dry-run     # which evals does this change affect?
terrain ai doctor                        # validate setup
terrain ai findings --json               # CI-consumable eval-gap findings
```

`terrain report pr` flags changed AI surfaces with no covering eval. The dependency graph that powers test selection for application code also traces AI surface edges.

## Hardening a prompt before deployment

Two generators produce a test suite around a prompt without invoking the model — the assertion is yours:

```bash
# Boundary-case mutation tests from a JSON Schema
terrain scaffold --schema schemas/input.json --prompt prompts/main.md \
  > tests/test_prompt_boundaries.py

# Jailbreak-shaped inputs matched against the prompt body
terrain inject --prompt prompts/main.md \
  > tests/test_prompt_injection.py
```

Default output is pytest; use `--lang typescript` for vitest.

## Downstream tooling handoff

Every `terrain analyze` writes a canonical `.terrain/findings.json` (schema version 1). `terrain mcp` reads it; IDE plugins consume it; SARIF uploaders transform it.

## What Terrain reads out of the box

- **Frameworks** — Jest, Vitest, Mocha, Playwright, Cypress, pytest, Go testing, JUnit 5, RSpec, and more (full list in [`docs/compatibility.md`](compatibility.md))
- **Languages** — Python, TypeScript / JavaScript, Go, Java, Ruby
- **Schema sources** — Postgres / MySQL DDL, Pydantic, TypeScript types, sqlc, gorm, Prisma, sqlalchemy
- **Eval frameworks** — Promptfoo, DeepEval, Ragas
- **Pipelines + registries** — dbt, Airflow, Prefect, MLflow, Weights & Biases

## Next

- [CLI specification](cli-spec.md) — every command, every flag
- [Example reports](examples/) — full sample outputs for analyze / impact / insights / explain
- [Signal catalog](signal-catalog.md) — every detector, what it fires on
- [Contributing](contributing/adding-a-measurement.md) — extending Terrain
