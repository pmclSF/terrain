# Terrain for frontend teams

If you ship UI in React / Vue / Svelte / Solid / Angular, you probably
have a mix of:

- **Unit tests** — Vitest or Jest, hitting hooks, reducers, utilities.
- **Component tests** — React Testing Library, Vue Test Utils, in-browser
  Cypress / Playwright Component Testing.
- **End-to-end tests** — Cypress, Playwright, WebdriverIO running real
  browser flows.
- **Visual regression** — Percy, Chromatic, Storybook test runners.

Terrain treats this stack as one *test surface* and tells you where the
weak points are.

## What Terrain catches that you'd miss otherwise

- **E2E redundancy** — three Cypress flows that exercise overlapping
  UI states. Surfaces as `duplicateTestCluster` with the redundant
  scenarios listed.
- **Mock-heavy unit tests** — components mocked so deeply the test
  validates the mock instead of the component (`mockHeavyTest`).
- **Snapshot saturation** — components with ten snapshots and zero
  behavioural assertions (`snapshotHeavyTest`). Snapshots are valid;
  ten of them on one component usually isn't.
- **Coverage blind spots** — exported components with no test, or
  tested only at the E2E layer (slow, brittle, expensive
  feedback). Surfaces as `coverageBlindSpot` with severity scaled by
  the component's import fan-in.
- **Conversion blockers** — Jest tests using globals (`jasmine.*`,
  `sinon.*`) that don't translate cleanly to Vitest, with per-file
  confidence reporting before you start migrating.
- **AI surface coverage** — if your frontend embeds prompts (chat
  widgets, streaming UI, RAG explainers), Terrain inventories them
  alongside the conventional surfaces and flags the ones with no eval
  scenario.

## A typical workflow

```bash
# 1. Get a structural read of the suite.
terrain analyze --root . --json | jq '.testsDetected'

# 2. Find what to fix first.
terrain insights --root . --detail 2

# 3. On a PR, see what your change actually affects.
terrain impact --base main --json

# 4. Plan a Jest → Vitest migration.
terrain migration readiness --from jest --to vitest --root .
```

## Suggested CI hookup

```yaml
# .github/workflows/terrain.yml
name: terrain
on:
  pull_request:
    paths:
      - 'src/**'
      - '**/*.test.{js,jsx,ts,tsx}'
      - '**/*.spec.{js,jsx,ts,tsx}'

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - run: npx mapterrain analyze --json --report-out terrain-report.json
      - run: npx mapterrain impact --base origin/main --json
      - uses: actions/upload-artifact@v4
        with:
          name: terrain-report
          path: terrain-report.json
```

## What Terrain doesn't do for frontend specifically

- It doesn't run your tests. Your existing Vitest / Jest / Playwright
  invocation continues to be the runner.
- It doesn't know about your design system. A `Button` with no test
  is an `untestedExport` regardless of whether your design system
  considers it stable.
- It doesn't render components. Visual regression remains the job of
  Chromatic, Percy, or Storybook.

## Where to go next

- `docs/cli-spec.md` — full command reference.
- `docs/signal-catalog.md` — every signal type and when it fires.
- `docs/severity-rubric.md` — what Critical / High / Medium / Low /
  Info actually mean.
