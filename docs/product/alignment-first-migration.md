# Migration is alignment-first, not conversion-first

How Terrain frames test framework migration in 0.2 — and why
"converge on a framework of record" is a more useful question than
"convert N files from framework A to framework B."

## The framing shift

Pre-0.2 Terrain talked about framework migration as conversion: a
mechanical transform from Mocha to Jest, from unittest to pytest,
from JUnit 4 to JUnit 5. That framing is correct but incomplete. It
optimizes for the engineer running a single conversion, not for the
team trying to bring a 50-repo portfolio into a coherent shape.

The 0.2 launch-readiness review surfaced the disconnect: most teams
care less about *converting* and more about *aligning*. They have
six different test frameworks across twelve services, the new hires
don't know which one their team uses, the CI templates fork along
framework lines, and the cost of the inconsistency dwarfs the cost
of any individual conversion.

So 0.2's migration framing leads with alignment:

> **Step 1: declare a framework of record per repo.**
> **Step 2: see where each repo drifts from its declared framework.**
> **Step 3: converge gradually — Terrain helps you sequence the work.**

Conversion (the mechanical transform side) is one of the tools you
use *during* convergence. It's not the headline; the headline is
the convergence itself.

## What this looks like in practice

### Single repo

```bash
# Declare what this repo officially uses:
cat > .terrain/frameworks.yaml <<EOF
frameworksOfRecord:
  - jest        # primary unit
  - playwright  # primary e2e
EOF

# Run analyze. The output now includes a Framework Drift section:
terrain analyze
```

```
Framework Drift
------------------------------------------------------------
  Of-record:  jest, playwright
  Detected:   jest (842 files), mocha (47 files), cypress (12 files)

  ⚠ 47 mocha test files (5.4% of suite) drift from of-record framework
  ⚠ 12 cypress test files in e2e/ — possibly intentional but undeclared

  See: terrain migrate convergence-plan --target jest --source mocha
```

The drift section is alignment-first: it answers "where am I
inconsistent" before it answers "what conversion do I run."

### Multi-repo

```bash
# Declare the portfolio:
cat > .terrain/repos.yaml <<EOF
version: 1
description: Acme engineering test alignment
repos:
  - name: web-app
    path: ../web-app
    frameworksOfRecord: [jest, playwright]
  - name: api-service
    path: ../api-service
    frameworksOfRecord: [pytest]
  - name: legacy-portal
    path: ../legacy-portal
    frameworksOfRecord: [mocha, cypress]    # legacy stack, not migrating yet
EOF

# Run portfolio over the manifest:
terrain portfolio --from .terrain/repos.yaml
```

The portfolio output ranks repos by drift magnitude, surfaces
shared blockers, and proposes a convergence sequence. The single-
file converters are still there — they're step three, not step one.

## Why this matters for tier framing

Each conversion direction (Mocha → Jest, unittest → pytest, etc.)
ships with tier metadata indicating how confident the conversion
is in 0.2.0:

| Conversion direction       | Tier in 0.2.0 | Notes |
|----------------------------|---------------|-------|
| Mocha → Jest               | Stable        | Top-3; conversion-corpus calibrated |
| Jasmine → Jest             | Stable        | Top-3; conversion-corpus calibrated |
| Vitest → Jest              | Stable        | Top-3; near-trivial transform |
| unittest → pytest          | Experimental  | Common case works; class-based fixtures partial |
| JUnit 4 → JUnit 5          | Experimental  | `@Test` swap solid; `@RunWith` cases partial |
| Mocha → Vitest             | Experimental  | Lower demand; smaller corpus |
| Cypress → Playwright       | Experimental  | E2E selectors don't transform 1:1 |
| Other                      | Tier 3 / preview | Use at your own risk; preview only |

`terrain migrate list` surfaces the tier per direction so adopters
see the trust posture before they start a convergence run.

## What changes in CLI output

Two surfaces gain alignment-first framing:

### `terrain analyze`

The Framework section now includes a "Drift" subsection when a
`frameworksOfRecord` declaration is present in the repo. Without
the declaration, the legacy framework-distribution section
unchanged.

### `terrain migrate list`

Each conversion direction prints with a tier badge:

```
Available conversions
  jest ← mocha       [Stable]      conversion-corpus calibrated
  jest ← jasmine     [Stable]      conversion-corpus calibrated
  jest ← vitest      [Stable]      near-trivial transform
  pytest ← unittest  [Experimental] class-based fixtures partial
  junit5 ← junit4    [Experimental] @RunWith cases partial
  vitest ← mocha     [Experimental] lower demand, smaller corpus
  ...
```

## What's still in flight (0.2.x and 0.3)

- **Per-direction conversion-corpus calibration to A-grade** for
  the top-3 stable directions (Track 6.7 of the parity plan)
- **`terrain portfolio --from <manifest>`** end-to-end aggregation
  of multi-repo drift — manifest format ships in 0.2 (Track 6.1);
  the aggregator lands in 0.2.x (Track 6.2/6.3)
- **Cross-repo policy aggregation** — apply one policy to N repos
  via the portfolio manifest. 0.3 work.

Until those land, the alignment-first reframing is a **doc and CLI
output change**, not a brand-new aggregator. The single-repo
framework-drift section ships in 0.2; the multi-repo aggregator
that consumes the manifest ships when it ships.

## Anti-goals

- We do not auto-convert files in 0.2. `terrain migrate run` is
  preview-by-default; users review the diff and apply manually
  (or with their own tooling).
- We do not declare a "best" framework. The framework-of-record is
  a per-team declaration; Terrain doesn't have an opinion about
  Jest vs. Vitest vs. Mocha. We surface drift relative to your
  declaration, not relative to ours.
- We do not block convergence on calibration. Adopters can run
  experimental-tier conversions today; the tier badge is honest
  signaling, not a gate.

## Related reading

- [`docs/product/vision.md`](vision.md) — full pillar narrative
  (Align is the secondary pillar in 0.2)
- [`docs/release/feature-status.md`](../release/feature-status.md) —
  per-capability tier matrix
- [`internal/portfolio/manifest.go`](../../internal/portfolio/manifest.go) —
  the multi-repo manifest schema
- [`docs/architecture/27-go-native-conversion-migration.md`](../architecture/27-go-native-conversion-migration.md) —
  conversion engine architecture (the mechanical transform side)
