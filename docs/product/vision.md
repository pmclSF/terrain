# Terrain — Product Vision

> **Terrain is the control plane for your test system.**
>
> It maps how your unit, integration, e2e, and AI tests actually relate
> to your code — and lets you gate changes based on that system as a
> whole.
>
> See what's covered, what's missing, and what's overlapping.
> See which tests matter for a PR — and why.
> Bring AI evals into the same review pipeline as the rest of your
> tests.

This document is the durable north-star for Terrain. Each release
updates the trajectory section; the headline pitch and the three-pillar
shape stay stable across releases.

---

## The user's actual job

A staff engineer / platform engineer / tech lead inherits or grows a
codebase. They need to answer:

1. *Do I understand what testing exists across this codebase?*
2. *Does the testing surface align with the code surface, or has it
   drifted?*
3. *When something changes, what does it actually put at risk?*
4. *Are AI features in this system tested with the same rigor as the
   rest, or are they a blind spot?*
5. *Across our repos, is testing uniform, or is each team's posture
   invisible to the others?*
6. *Can I gate on all of that in CI without writing five different
   integrations?*

No single tool answers more than two of those questions today.
**Terrain's job is unifying the answer.**

## What Terrain is

A typical product team's test universe lives across five different
runners (Jest / pytest / Go test / Playwright / Promptfoo et al.),
three different report formats, and zero unified gates. **Terrain
is the layer above** — it doesn't execute tests, it understands them
and gates against them.

Two phrases doing real work:

- **"Control plane"** — Terrain operates one layer above the test
  runners. Same architectural pattern as a Kubernetes control plane,
  but for the test system. Test runners continue to execute; Terrain
  reads what they produce, models the system, and decides what's
  blocking.
- **"As one thing"** — the unification value. The PR risk-report
  doesn't care whether a finding came from a flaky unit test, a
  missing AI eval, or a coverage gap. It's all the same finding shape
  with the same severity model and the same suppression workflow.

## The three pillars

Each pillar is *what you do with the model*. The pillars share a
common substrate (snapshot, signal model, CI gate primitives) and a
common surface (uniform exit codes, JSON contract, severity model,
suppression file).

| Pillar | Job | External framing |
|--------|-----|------------------|
| **Understand** | See the test universe as one thing | "See what's covered, what's missing, what's overlapping" |
| **Align** | Reduce drift between code, tests, and repos | "Standardize and reduce drift across test systems" |
| **Gate** | One CI gate over the whole system | "Gate PR changes based on the system as a whole" |

### Understand — see the test universe as one thing

Map every test, framework, code unit, AI surface, eval scenario,
ownership boundary, coverage gap, and runtime metric into one
structural model. Surface the alignment between what's tested and
what matters: coverage relative to complexity, density relative to
risk, AI surfaces relative to eval coverage. Diff the model over time
to see what changed.

Capabilities: `terrain analyze`, `terrain report
summary/posture/metrics/focus/insights/explain`, `terrain compare`,
AI surface inventory, `terrain serve` (local view), `terrain debug *`
(diagnostics), `terrain portfolio` (cross-repo view).

### Align — reduce drift between code, tests, and repos

When the testing surface doesn't match the code surface — exports
without tests, frameworks fragmenting across directories, AI surfaces
shipping without scenarios, one team's posture diverging from
another's — Terrain shows where the drift is and what it would take
to converge. When convergence requires framework migration (Jest →
Vitest, Mocha → Jest, JUnit 4 → 5, etc.), Terrain does that with
per-file confidence and a preview-before-apply workflow.

**Migration is a mode of alignment, not a separate product.** The
docs lead with "your repo has framework drift; here's what it would
take to converge", not "convert this file."

Capabilities: `terrain migrate` namespace, `terrain convert` per-file,
conversion-history audit trail, alignment views in `posture` /
`portfolio`, `terrain report select-tests` (test-set alignment to a
change).

### Gate — bring everything under one CI gate

Whether the underlying test is a unit test, an eval scenario, an AI
risk signal, a policy violation, or a coverage threshold breach, the
CI experience is the same: same `--fail-on`, same
`--new-findings-only`, same suppression model, same exit codes, same
JSON contract. The PR comment template is one template. The CI
workflow is one workflow.

Capabilities: `terrain report pr`, `terrain report impact`, `terrain
ai run --baseline`, `terrain policy check`, `--fail-on` /
`--timeout` / `--new-findings-only` flags, suppressions, stable
finding IDs, per-finding remediation pointers.

## The unifying thread

The pillars feel like one product because they share **CI gate
primitives**:

| Primitive | Used by |
|-----------|---------|
| Exit-code conventions (0/1/2/4/5/6) | every command |
| `--fail-on <severity>` | analyze, pr, impact, ai run |
| `--new-findings-only --baseline <path>` | analyze, pr |
| `.terrain/suppressions.yaml` | every detector |
| Stable finding IDs | every signal |
| `--format json/sarif/annotation` | every read-side command |
| One PR-comment template (`changescope`) | impact, pr, ai run |

A CI engineer who learns the gating model once doesn't relearn it
for AI evals or for cross-repo alignment.

## What's distinctive

Compared to tools your audience already knows:

| Compared to | Terrain's distinct position |
|-------------|------------------------------|
| Jest / Vitest / pytest / Go test / Playwright | Reads them, doesn't replace them — operates one layer above |
| Coverage tools (Istanbul, gcov, coverage.py) | Ingests coverage as evidence; doesn't instrument code |
| SonarQube / Semgrep / CodeQL | Source-side bug-finding is theirs; *test-system* quality is ours |
| Promptfoo / DeepEval / Ragas | They run AI evals; we ingest the results into the same gate as everything else |
| Test-impact tools (Bazel, Gradle test queries) | Cross-language structural impact, not single-toolchain |
| AI safety / runtime guard tools (Lakera, Guardrails) | Structural / pre-deploy / inventory; they're runtime |
| GitHub code scanning | We emit SARIF *into* it; we don't compete |

**The unique claim:** the only tool that gives you one model of your
test universe across test types, languages, and repos, with CI
gating primitives.

## What Terrain explicitly isn't

- **Not a test runner.** Test runners continue to execute; Terrain
  reads the artifacts they produce.
- **Not a coverage tool.** Coverage data is ingested as evidence,
  not computed.
- **Not a static analyzer for application code.** Terrain inspects
  *test* code structure (assertions, mocks, framework patterns,
  scenario coverage). Source-side bug-finding stays with Sonar /
  Semgrep / CodeQL.
- **Not an LLM eval framework.** We ingest Promptfoo / DeepEval /
  Ragas output; we don't run prompts against models.
- **Not a developer-productivity dashboard.** No per-developer
  metrics, no leaderboards. Ownership data routes work and doesn't
  score people.
- **Not a SaaS.** Local-first, CI-native, no account, no telemetry
  off-host. (Note: `npm install -g mapterrain` and `brew install`
  download signed binaries from GitHub Releases as part of
  installation; analysis itself does not phone home.)
- **Not an LLM safety service.** AI risk detectors are heuristic and
  recall-anchored; precision floors against labeled corpora are 0.3
  work.

## Anti-goals (0.2.x)

These are explicit non-claims for the 0.2 line. They exist to keep
execution honest:

- **Terrain does not guarantee safe test skipping.** It provides
  *explainable* selection and gating signals. The "see which tests
  matter — and why" pitch line is a clarity claim, not a safe-skip
  claim.
- **Terrain does not run your tests.** Bears repeating.
- **Terrain does not judge model truthfulness.** AI risk detectors
  surface heuristic structural patterns and ingest eval-framework
  metadata.
- **Terrain does not promise public-grade precision floors in 0.2.x.**
  Recall-anchored calibration on the 27-fixture corpus is the only
  honest claim until labeled-real-repo precision corpora ship in 0.3.

## Trajectory

Three releases, with the verb sharpening at each step:

| Release | Verb | What ships |
|---------|------|------------|
| **0.2.0** | "See clearly + gate progressively" | Three pillars at parity floors (Gate ≥ 4, Understand ≥ 3, Align ≥ 3 soft); suppressions; finding IDs; `--new-findings-only`; AI risk subdivision into inventory/hygiene/regression; multi-repo manifest; alignment-first migration framing; per-area examples; design system; per-detector "known false positives" docs |
| **0.3** | "Take control" | Labeled-corpus precision floors per detector; AST taint flow for prompt injection; suppression lifecycle (expiry, owner, audit); AI gate as standalone command; plugin architecture; sandboxing for eval execution; legacy CLI alias removal |
| **0.4** | "Test the universe" | AI-aware integration / e2e tests under the control plane (define them, run them in CI, gate on them, suppress them); cross-repo alignment workflows; eval-test composition (unit + integration + eval as one feature) |

**"Take control" only becomes a public claim at 0.3.** In 0.2.x,
public copy stays at "see clearly + gate progressively." Marketing
maturity tracks engineering maturity per the parity gate.

## Capability map

Every shipping capability has a pillar. Tier-1 capabilities are
publicly claimable; Tier-2 is shipping but explicitly experimental;
Tier-3 is in development, opt-in, no public claim.

| Capability | Pillar | Tier (0.2.0) |
|------------|--------|--------------|
| `analyze` | Understand | Tier 1 |
| `report insights / posture / summary / metrics / focus / explain` | Understand | Tier 1 |
| `compare` | Understand (over time) | Tier 1 |
| `terrain serve` | Understand (local view) | Tier 2 |
| `terrain portfolio` | Understand (multi-repo) | Tier 2 (emerging) |
| `terrain debug *` | Understand (diagnostics) | Tier 2 |
| `migrate` / `convert` | Align | Tier 1 |
| `report select-tests` | Align (test-set to change) | Tier 2 |
| `report pr` | Gate | Tier 1 |
| `report impact` | Gate (PR-scoped) | Tier 1 |
| `analyze --fail-on / --timeout / --new-findings-only` | Gate | Tier 1 |
| `policy check` | Gate (policy dimension) | Tier 1 |
| AI surface inventory | Understand | Tier 1 (reliable) |
| AI risk: hygiene + regression | Gate | Tier 2 (visible, not gating-critical) |
| Eval artifact ingestion | Gate | Tier 1 |
| `terrain ai run --baseline` | Gate (regression-aware) | Tier 2 |
| `terrain init` | onboarding (cross-pillar) | Tier 1 |

Nothing orphan, nothing hidden. The breadth stays; tiering is honest
about which capabilities make public claims at this release.

## Primary workflow

Two commands an adopter learns first, in order:

```bash
terrain analyze         # Understand your test system
terrain report pr       # Gate PR changes based on it
```

Everything else is a deeper view *off* this primary workflow. Docs,
help text, and the recommended GitHub Action snippet all anchor to
this two-step flow. Other commands (`migrate`, `portfolio`, `ai run`,
`policy check`, `serve`, `debug *`) are reachable but discovered
*through* this workflow, not as alternative entry points.

## How this document evolves

- The headline pitch and three-pillar shape are stable across
  releases. Changing them needs a strategy decision, not a doc edit.
- The trajectory table updates each release: today's verb moves
  forward; the new release's verb takes its slot.
- The capability map updates whenever a capability changes pillar or
  tier. Updates happen in the same PR that lifts the capability.
- The anti-goals stay until they're no longer true. Each anti-goal
  has a definite "this becomes a goal in release X" trigger; that
  trigger lives in the rubric (`docs/release/parity/rubric.yaml`).

The pitch is the source of truth that the README, quickstart, and
marketing copy point at. When this doc and the README disagree, this
doc wins until the README is updated.
