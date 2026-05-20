# Terrain for backend teams

Server-side codebases tend to have wide test surfaces: unit tests on
business logic, integration tests against real (or testcontainerised)
databases / queues / caches, contract tests against shared APIs, end-
to-end tests against staging. Terrain treats this stack as one model
and shows you the bottlenecks.

## What Terrain catches that you'd miss otherwise

- **Layered redundancy** — the same business rule asserted by a unit
  test, an integration test, *and* an E2E test, where the integration
  one would have caught the bug alone. Surfaces as
  `duplicateTestCluster` plus the redundancy is visible in the
  per-layer breakdown of `terrain insights`.
- **Cross-service blast radius** — a refactor touches `internal/auth/`
  and Terrain shows the 23 tests in 6 packages that exercise that path,
  not just the 3 in `internal/auth_test.go`.
- **Skipped tests in CI** — `t.Skip` / `@pytest.mark.skip` /
  `@Disabled` on tests that have been disabled longer than the
  configured budget (`skippedTestsInCI`).
- **Runtime budget creep** — slow tests from CI runtime artefacts,
  with the per-package breakdown so you know which suite to optimize.
- **Migration blockers** — pytest fixtures with implicit ordering,
  JUnit 4 patterns mid-migration to JUnit 5, custom matchers that
  don't translate (`migrationBlocker`, `customMatcherRisk`).
- **AI surface coverage** — for backends that include LLM agents,
  RAG pipelines, eval suites: every surface gets inventoried and
  Terrain flags the ones missing scenarios, hardcoded keys, floating
  model tags, and prompt-injection-shaped concatenation.

## A typical workflow

```bash
# 1. Structural read.
terrain analyze --root . --json > .terrain/snapshot.json

# 2. PR change scope (what does this diff actually exercise?).
terrain impact --base origin/main --root .

# 3. Decide what to fix this sprint.
terrain insights --root . --detail 2

# 4. Pick the right tests for a CI pipeline.
terrain select-tests --base origin/main --output testlist.txt
go test $(cat testlist.txt | tr '\n' ' ')
```

## Suggested CI hookup

```yaml
# .github/workflows/terrain.yml
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
      - run: |
          curl -L https://github.com/pmclSF/terrain/releases/latest/download/terrain_$(uname -s | tr A-Z a-z)_amd64.tar.gz \
            | tar -xz
          ./terrain analyze --root . --json > terrain-report.json
          ./terrain impact --base origin/main --json
      - uses: actions/upload-artifact@v4
        with:
          name: terrain-report
          path: terrain-report.json
```

## Pytest specifics

- Terrain understands `parametrize` (counts test instances, not just
  test functions).
- Conftest discovery is recursive across the tree.
- Fixture dependencies surface in `terrain insights` when a heavily-
  shared fixture is the bottleneck for a slow suite.

## Go specifics

- Hierarchical `t.Run` subtests are reported with full paths.
- Build tags are honoured — `//go:build integration` tests count as
  integration tier, not unit.
- Race detector counts as a runtime-data input when you pipe `go test
  -race -json` into the runtime artefact slot.

## JUnit / Java specifics

- `@Nested` classes decompose into the suite hierarchy.
- `@DisplayName` annotations populate the human-readable scenario name.
- Maven and Gradle project shapes both auto-detect.

## Where to go next

- `docs/cli-spec.md` — full command reference.
- `docs/signal-catalog.md` — every signal type and when it fires.
- `docs/calibration/CORPUS.md` — the labeled-fixture set we use to
  measure detector precision/recall, including backend-shaped fixtures.
