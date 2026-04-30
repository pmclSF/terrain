# Terrain vs. Codecov / SonarQube / Launchable

Three tools that get evaluated alongside Terrain often enough that the
boundaries deserve a written-down comparison. None of these are
direct replacements for each other — they sit at different layers of
the test system. Knowing which one solves your problem matters.

## TL;DR

| | Codecov | SonarQube | Launchable | Terrain |
|---|---|---|---|---|
| Coverage measurement | ✅ best-in-class | partial | — | ingests, doesn't compute |
| Source-code static analysis | — | ✅ best-in-class | — | test-code only |
| Test selection / impact | partial | — | ✅ best-in-class | yes, structural-graph driven |
| AI / LLM-eval signals | — | — | — | ✅ |
| Test conversion / migration | — | — | — | ✅ |
| Test signal vocabulary | — | — | partial | ✅ 56 types, calibrated |
| Local-first / OSS | partial | partial | — | ✅ |

## Codecov

**What Codecov does best:** measure code coverage. Instrument your
runtime, ingest reports, render diffable coverage views on PRs. If
you don't have coverage data, Codecov gets you there.

**What Terrain does that Codecov doesn't:**

- Coverage measurement is one input among many. Terrain takes coverage
  reports and combines them with test structure (assertion density,
  mock heaviness, framework patterns), the dependency graph (what's
  reachable from what), and runtime artefacts (flakiness, slow tests)
  to produce decisions: "weak coverage on `src/auth/handlers.go`
  *and* the existing tests are mock-heavy, so the surface is more
  exposed than the line-coverage number suggests".
- AI evals, prompt surfaces, agent definitions are first-class.
  Codecov has no model for these.
- Test conversion (Jest → Vitest, Cypress → Playwright, etc.) with
  per-direction confidence reporting.
- No vendor lock-in: runs locally, OSS, no paid tier.

**Where Codecov stays better:** if your only need is coverage as a
gate (`coverage > 80%`), and you want hosted UI / org rollups /
historical charts out of the box, Codecov is more direct.

## SonarQube

**What SonarQube does best:** static analysis on source code.
Vulnerability rules, code smells, cognitive complexity, technical
debt scoring across hundreds of rules per language. Mature, broad,
proven.

**What Terrain does that SonarQube doesn't:**

- SonarQube analyses application code. Terrain analyses the *test
  system around it*: test structure, scenario coverage, framework
  patterns, conversion blockers, AI surfaces. Different layer.
- AI-domain signals (`aiHardcodedAPIKey`, `aiPromptInjectionRisk`,
  `aiNonDeterministicEval`, ...) — Sonar's rule set isn't built for
  this.
- Migration readiness reports for test-framework changes.
- Per-detector calibration corpus measuring precision/recall openly.
- No SaaS, no licensing.

**Where SonarQube stays better:** application-code bug-finding,
language-level lint rules, security CWE coverage on source. Don't
replace Sonar with Terrain — run both.

## Launchable

**What Launchable does best:** ML-driven test selection. Predict the
subset of tests most likely to catch regressions on a given diff,
shrink CI time, hosted analytics.

**What Terrain does that Launchable doesn't:**

- Structural-graph impact analysis without a hosted ML model. Terrain
  builds the dependency graph from imports and code-unit relationships
  in the snapshot, then traverses to find which tests cover the
  change. Fully explainable: every impacted test cites the path.
- Signal vocabulary: 56 stable signal types beyond "ran / didn't
  run", with a documented severity rubric.
- AI surface inventory and AI-domain signals.
- OSS, local, no telemetry.
- Test conversion / migration readiness.

**Where Launchable stays better:** mature production deployment of
ML-based test selection across very large monorepos with extensive
historical run data; if you need flat-fee SaaS with a managed model
and don't want to operate the analysis locally, Launchable's offering
is more turnkey.

## When you'd use multiple

The most common stack is:

- **SonarQube / Semgrep** for source-code static analysis
- **Codecov / Coveralls** for coverage measurement on the runtime side
- **Terrain** as the test-system layer that ingests both and adds
  structural / AI / conversion analysis the others don't model

Terrain is intentionally narrow about what it claims (`What Terrain
Is Not` in the README enumerates the boundaries). If a comparison
question doesn't have a clean answer here, open an issue and we'll
add an entry.
