# Terrain — Product Plan

> *This is the canonical product reference for Terrain. Technical architecture detail lives in `DESIGN.md`; this document is the product story, the principles, and the load-bearing requirements they imply.*
> *0.2.0 is the first release with stability commitments. Pre-0.2.0 was unstable by design; 0.2.0 is a clean slate with no backward-compatibility guarantees from prior versions.*

## Quick navigation

1. [Mission](#1-mission) — what Terrain is, related work, two capability areas
2. [Audience](#2-audience) — five personas served
3. [Three co-equal product goals](#3-three-co-equal-product-goals) — unified graph + real CI gate + auditable quality
4. [Non-goals](#4-non-goals) — what Terrain explicitly isn't
5. [Vocabulary](#5-vocabulary) — canonical terms
6. [Principles](#6-principles) — decision-making heuristics
7. [Architecture](#7-architecture-three-surface-model) — CI / CLI / agent surfaces, snapshot pipeline, conversion subsystem
8. [Diagnostic format](#8-diagnostic-format) — Finding schema, four renderers, annotation cap
9. [Rule catalog](#9-rule-catalog) — 75 rules; 30 stable (ceiling, actual = those clearing LB-5) / 45 preview
10. [Configuration](#10-configuration--terrainyaml-v1) — `terrain.yaml` v1 schema
11. [LLM economics & security](#11-llm-economics--security) — three tiers, provider matrix, Ollama default
12. [Quality requirements](#12-quality-requirements-load-bearing) — LB-1, LB-2a/b/c, LB-3, LB-4, LB-5 (Wilson), LB-6 (recall), LB-7, LB-8, LB-9 (per-phase), LB-10, LB-11 (bidirectional), LB-12 (decision-maker)
13. [Validation harness](#13-validation-harness) — readiness cards, calibration corpus, public artifacts
14. [Dogfood repos](#14-dogfood-repos) — five repos, sourcing plan
15. [Rule-doc template](#15-rule-doc-page-template) — 11-section canonical structure
16. [0.2.0 scope](#16-020-scope) — must-ship, non-goals, CLI surface
17. [Roadmap](#17-beyond-020--phased-roadmap) — phased integration philosophy
18. [Project operations](#18-project-operations) — license, governance, versioning, telemetry, response, doc map
19. [Open questions](#19-open-questions) — operational items remaining
20. [Glossary](#20-glossary)

---

## 1. Mission

**Terrain is a pre-flight check for AI features.** It is a static analyzer that runs locally and in CI, with **no LLM API key required**, ever. It treats unit tests, integration tests, end-to-end tests, ML model evaluations, data validation, and LLM eval scenarios as nodes in **a single dependency graph**, and surfaces failures in that graph as a **real CI gate** — not as a comment, dashboard, or email.

The headline use case: a frontend developer changes a form field and learns, *in their PR*, that their change will degrade a downstream AI model's behavior — before merge. The reverse is equally true: an ML engineer who edits a prompt or retrains a model learns which upstream code paths and downstream features are affected. Terrain achieves this without ever calling an LLM at scan time: the analysis is structural (import-graph, AST, schema parsing), not generative.

Terrain exists because the unified-graph + CI-gate combination doesn't exist in any one tool today. The closest neighbors each cover part of the territory:

- **Eval frameworks** (promptfoo, deepeval, ragas, gauntlet, Great Expectations, Evidently, deepchecks, Fairlearn) understand AI/ML quality and produce verdicts — but they don't see the code structure, don't participate in the merge gate as test cases, and don't cross to upstream causes when an eval regresses.
- **Code review and PR-assistance tools** (CodeRabbit, Greptile, Sourcegraph reviews) read code and produce review comments — but they don't run evals, don't gate on AI behavior regressions, and don't appear as native test cases in the CI test pipeline.
- **Test runners and impact-analysis platforms** (Codecov, Coveralls, Bazel test-impact, Nx affected-graph) understand code-to-test relationships — but they're AI-blind, and most don't surface as descriptive diagnostics in the merge gate.
- **Data-lineage tools** (dbt lineage, OpenLineage, Marquez) understand data-flow — but stop at the SQL or pipeline boundary, don't extend to AI surfaces, and don't produce gate-blocking findings.
- **MLOps platforms** (MLflow, Weights & Biases, SageMaker, Vertex AI) track experiments, registries, and deployments — but they don't see the application code that uses them, and don't participate in pre-merge CI.
- **Static analysis platforms** (Semgrep, CodeQL, Sonarqube) find code-level issues — but their model is per-file, not graph-aware, and they don't model AI surfaces or evals.

The category Terrain occupies — *unified code+AI/ML testing surfaced as a native CI gate* — sits in the gap between these. The relationship to neighbors is *composition* where possible (integrates with eval frameworks, reads from registries, emits findings into the same CI test pipeline as other test runners) and *partial replacement* where unavoidable (the AI/ML CI-gate use case overlaps with what AI-aware code-review bots do today; for that specific use case Terrain is an alternative, not an integration). The doc names these explicitly in §17's "out-of-scope integrations" list.

### What 0.2.0 is for

0.2.0 is the foundational release. It commits to a specific scope, quality bar, and trust profile that subsequent releases extend without redoing. Because no comparable unified tool exists today, the 0.2.0 release also serves as a reference for what "unified testing for AI/ML systems" looks like in practice — what the rule catalog covers, how the diagnostic format reads, what trust artifacts ship publicly. Marketing copy and external positioning can speak in stronger category-creation language; this document keeps the framing grounded so that quality decisions stay measurable.

### Two related capabilities sharing infrastructure

Terrain's deep understanding of test systems enables two capability areas built on the same foundation:

1. **AI/ML testing in CI** (the primary focus of this plan) — the unified-graph + CI-gate product described above.
2. **Test-framework conversion and migration** (a related capability) — Terrain converts test files between frameworks (Jest↔Vitest, Cypress↔Playwright, Mocha↔Jest, JUnit 4↔5, TestNG↔, pytest↔unittest, and others). This subsystem ships in the same binary, exposes its own CLI commands (`convert`, `migrate`, `detect`, etc.), and uses the same AST and graph infrastructure. It is positioned as a parallel capability with its own product story; this document does not detail it.

---

## 2. Audience

Terrain serves five personas, all working in the same repository:

- **Frontend developer.** Doesn't know the AI codebase; needs to know if their change breaks an AI contract.
- **Backend / platform engineer.** Owns the data layer; needs to know which models are affected by a schema or API change.
- **ML engineer working with classical ML.** Changes feature engineering or trains a new model; needs regression detection on test-set metrics, drift, and fairness.
- **ML engineer working with LLMs.** Edits prompts, RAG pipelines, or model fixtures; needs scenario-based regression detection.
- **Senior decision-maker (CTO, Principal Engineer, PM).** Evaluating Terrain for adoption; needs to understand scope, trust, FP cost, and stability guarantees.

The first four interact with Terrain through failing tests and PR diagnostics. The fifth reads the docs and the readiness cards. The product surface has to serve all five.

### Languages analyzed

Terrain's detector engines analyze: **Python, TypeScript, JavaScript, Go, Java**. Code in other languages (Rust, Kotlin, Swift, C / C++, Ruby, etc.) is not currently analyzed — `terrain analyze` will emit findings on Python/JS/Go/Java build tooling within those repos but not on the primary language code. Additional language coverage is future work.

---

## 3. Three co-equal product goals

All three are required. Any one alone collapses the category claim:

### Goal 1 — Unified graph
Code testing and AI/ML testing live in one dependency graph that spans the full stack: frontend, backend, data layer, pipelines, models, evals. Impact propagation and failure attribution cross language and system boundaries.

### Goal 2 — Real CI gate
Terrain's verdicts appear as a required-able status check, native test cases in the platform's Tests tab, inline annotations on changed lines, and locally-reproducible failures. **Not as a comment, not as an email, not as a dashboard.**

### Goal 3 — Auditable quality
Every claim Terrain makes is independently verifiable. Per-rule false-positive rates, triage benchmarks, and runtime measurements are *published* (not just internally tracked) per release. The labeled corpus the project calibrates against is public. Performance benchmarks include reproducible methodology. The trust profile is not a marketing claim; it's an artifact adopters can inspect before adoption and after.

A unified graph without a CI gate is a smarter dashboard. A CI gate without the unified graph is just another linter. Both without auditable quality is a marketing-trust play that fails the first time a senior engineer asks "where are your numbers." All three together are the product.

---

## 4. Non-goals

Explicit non-goals — Terrain is not these things, and the docs/positioning should not imply otherwise:

- **Terrain is not an eval framework.** It does not replace promptfoo, deepeval, ragas, gauntlet, Great Expectations, Evidently, Fairlearn, or deepchecks. It integrates with them.
- **Terrain is not a model registry.** It integrates with MLflow / W&B / SageMaker / Vertex registries; it does not replace them.
- **Terrain is not a CI platform.** It runs inside GitHub Actions, GitLab CI, CircleCI, etc.; it does not replace them.
- **Terrain is not a code-review bot.** It emits test cases and status checks, not PR-comment summaries of what the code does. The distinction from CodeRabbit / Greptile / similar: those tools generate narrative reviews of code quality intended for human reading; Terrain generates failing test cases with deterministic, structured diagnostics intended for the same workflow as `pytest` or `cargo test` output. Terrain's inline annotations on changed lines mark *cause locations for failing checks*, not opinions about code quality. A code-review bot wants the reviewer to read its commentary; Terrain wants the failing check to be self-explanatory enough that no commentary is needed.
- **Terrain is not a dashboard product.** Dashboards are downstream of the CI primitive; they are not the primary surface.
- **Terrain is not a hosted service.** It is an OSS tool that runs in the adopter's CI and on developer machines. No data leaves the adopter's infrastructure by default.

---

## 5. Vocabulary

Terrain's external positioning uses **"AI testing in CI"** as the headline phrase because "AI" is the industry term that drives discovery. The architectural scope is broader: classical ML, deep learning, LLMs, RAG, data pipelines, fairness, drift, performance.

**Avoiding the discovery problem for classical-ML adopters.** The marketing phrase risks classical-ML adopters bouncing off — they may not see "AI testing" as relevant to their sklearn / XGBoost / PyTorch work. The doc, README, and OVERVIEW make the broader scope explicit: subheadings, persona descriptions, and the rule catalog (with 5 classical-ML stable rules at 0.2.0) all communicate "AI and ML, not just LLMs." External landing-page copy can lead with "AI" and clarify with "(including classical ML, deep learning, and LLM systems)" — the headline is short for SEO; the body sentence is honest.

Canonical terms, used throughout this doc and the codebase:

| Term | Meaning |
|---|---|
| **Surface** | A point in the codebase where an AI/ML system is exposed. LLM call sites, model inference endpoints, feature pipelines, prompt templates, training scripts. Note: the *internal* graph model also tracks a `CodeSurface.Kind` field (low-level shape: `prompt`, `dataset`, `tool_definition`, `retrieval`, `handler`, `route`, `class`, etc., per existing `internal/models/code_surface.go`); the `surfaces:` block in `terrain.yaml` exposes a higher-level user-facing `type:` enum (`llm`, `classical_ml`, etc., per §10). The two taxonomies serve different purposes: `Kind` is detection-output; `type` is adopter-declared. They coexist; only `type` is part of the public `terrain.yaml` schema. |
| **Eval** | An oracle that produces a verdict on a surface's behavior. Includes LLM scenarios (promptfoo-style), test-set evaluations (sklearn-style), data-validation tests (Great Expectations-style), drift checks, fairness checks. |
| **Metric** | A score produced by an eval — rubric score, accuracy, F1, AUC, RMSE, KL divergence, calibration error, latency, cost. |
| **Finding** | A single result from one rule. Includes location, cause path, evidence, suggestions. Renders to four surfaces. |
| **Rule** | A configurable detection capability with a stable ID, severity default, and documentation. Adopters tune rules via `terrain.yaml`. |
| **Tier** | Stable or preview. Stable rules are validated to the LB bar; preview rules are catalog-complete but lower-validation. |
| **Cause path** | The chain of nodes through the unified graph connecting the change in the PR to the finding's primary location. |

Terms not used in code or docs: "scenario" (replaced by "eval"), "rubric score" (replaced by "metric"), "AI surface" (replaced by "surface"). Existing references are renamed in 0.2.0 as a direct rename — no alias period, because 0.2.0 is the first release with stability commitments.

---

## 6. Principles

These are decision-making heuristics, applied consistently:

### "Would pytest do this?"
The reference UX for any Terrain surface is the existing test-runner ecosystem: `pytest`, `cargo test`, `go test`, `jest`, `selenium`. If a proposed feature would not appear in those tools' default behavior, it does not appear in Terrain's. Anti-examples: PR comments summarizing passing tests, email digests, risk-score banners, executive-dashboard verbiage *on the PR surface*. The posture-band concept itself survives — it just lives in the Step Summary on the run page (audit-detail), not on the PR.

### Silence on green
When all tests pass, no signals appear on the PR surface. Full detail lives one click into the run — Step Summary, Tests tab, run logs, artifacts — and is fully auditable. But the PR itself is quiet: no comment, no annotation, no email beyond the platform default. The presence of a green status check is the entire signal on the PR.

### Descriptive but deterministic in CI
CI diagnostics are detailed, structured, and modeled on `cargo`, `mypy`, and `eslint` — not on bot comments. They include location, what changed, why it matters, behavioral evidence (where applicable), and a suggested action. They are templated, not LLM-rendered. No LLM call ever runs in the CI surface.

### FP > FN with cheap triage (for detection rules)
*Detection* rules — regression, hygiene, security, data-quality — bias toward false positives over false negatives. This bias is only survivable if each finding is cheap to triage: a developer must be able to look at the line, read the diagnostic, and decide "yes" or "no, here's why" in under 60 seconds. Every finding is specific, located, reproducible, and actionable.

*Coverage* rules describe state, not per-PR judgments ("this surface has no eval"; "this code has no test"), so the FP/FN frame doesn't apply. Coverage rules instead bias toward *completeness over precision* — flag every gap, accept that some gaps are intentional, give adopters easy per-path ignore configuration to suppress the intentional ones.

### Reproducibility parity
Every CI failure can be reproduced by running `terrain test <selector>` locally on the developer's machine, producing the same diagnostic output. If a failure cannot be reproduced locally, the gate is broken.

### Stable APIs from 0.2.0 release
Rule IDs, JSON output shape, `terrain.yaml` schema, CLI flags, and the artifact format become public APIs **at the 0.2.0 release**. Pre-0.2.0 was unstable by design. Post-0.2.0, breaking changes go through a one-cycle deprecation. The 0.2.0 release is the clean-slate moment — we make every correctness fix now, freely, because no stability commitments are in force until the release tag lands.

**Single source of truth for the stability contract:** the detailed deprecation mechanics (alias period, `version: 1` JSON field, stderr deprecation messages, removal in the next-minor-version-after) live in §18 versioning. §8 rule IDs, §10 JSON / YAML schemas, and §12 LB-6 all reference §18 rather than restating. The contract is one thing in one place.

### Correctness over schedule
Terrain releases ship when the LB quality bars are met, not on a fixed timeline. We do not ship rules at lower validation than the LB requires. We do not ship the gate at lower diagnostic quality than the design specifies. We do not lower a quality bar to make a release date. The correct response to schedule pressure is to reduce *scope* (defer rules from stable to preview, or from preview to a later release) — never to lower the bar on what does ship. A release that ships later at full quality is always better than one that ships sooner with compromises that erode adopter trust.

### OSS-only dependencies
Terrain integrates with OSS libraries only. No proprietary code dependencies, no commercial SDK lock-in, no library that requires a vendor relationship to use. Commercial *APIs* (OpenAI, Anthropic, etc.) are acceptable as opt-in BYOK paths because adopters bring their own keys — the project itself doesn't take a dependency on the vendor. This principle excludes integrations like vendor-only ML governance suites or proprietary eval frameworks; the supported integration set in §16 / §17 reflects this constraint.

### Solo execution
Terrain is built by a single engineer with AI-assistant collaboration, not a team. This is the engineering-capacity reality and the plan is written to be executable solo. Practical consequences: work proceeds sequentially through the Tier 0→1→2→3→4 dependency-order spine; parallel workstreams across the same Tier are possible (with AI assistance) but parallel work across Tiers requires the spine to be honored; release pace is bounded by single-engineer attention, not team coordination. *Correctness over schedule* applies even more sharply solo — there is no engineering reserve to absorb dropped quality.

---

## 7. Architecture: three-surface model

Terrain has three surfaces. Each consumes the same underlying artifact, varies in interactivity, and houses the LLM (or doesn't) differently.

### CI surface
- Renders to: JUnit XML, GitHub check-run annotations, Step Summary, status check
- Interactivity: one-shot, passive
- LLM: **never.** All diagnostics are templated.
- Cost: free, no setup, no provider key
- Audience: every adopter; required-able as a branch protection check

### CLI surface (`terrain test`, `terrain explain`)
- Renders to: terminal output (cargo-style), JUnit (same as CI), JSON
- Interactivity: developer-driven; `terrain explain` supports follow-up questions
- LLM: **Ollama is the documented default** per §11 (silent default written by `terrain init`); alternatives: BYOK external (OpenAI / Anthropic / etc.), internal OpenAI-compatible endpoint, or skip entirely. Configurable via `explain:` in `terrain.yaml`.
- Cost: free with Ollama (local compute) or template-only; external-API cost is on the adopter's account
- Audience: developers debugging a failure locally; LLM-banned orgs use it template-only
- **Relationship to CI surface when LLM is disabled:** when the CLI runs with `explain.provider: none`, `terrain explain` output and the CI Step Summary for the same finding render the same templated diagnostic — the value of CLI in this mode is *local reproducibility* (LB-4) and machine-readable output (`--json`), not enrichment. Enrichment comes from the LLM tier; that's the differentiator. The "three-surface" framing is honest only because the LLM tier exists — without it, CLI≡CI for adopters who choose template-only.

### Agent surface (MCP server)
- Renders to: MCP tool responses consumed by Claude Code, Cursor, ChatGPT-with-Apps-SDK, future agents
- Interactivity: conversational
- LLM: the agent's LLM (the user-or-org's existing subscription — for solo / OSS use, the user's; for enterprise, the org's corporate AI account)
- Cost: free to Terrain; the user-or-org pays their existing AI provider
- Audience: developers already working in an AI assistant; marketplace listings (Claude Skills, OpenAI) are wrappers around the MCP server, not separate products

### The artifact-as-handoff contract
All three surfaces read the same artifact: a JUnit XML file + a structured `findings.json` + the dogfood-repo state at the failing commit. The artifact is the contract. The CI surface produces it; the CLI and agent surfaces consume it. `terrain explain --from-run <id>` reads it directly — no local reproduction required to debug.

This decoupling means:
- The CI surface can ship at full quality without any LLM features ever existing.
- The CLI and agent surfaces are *additive* — they enrich without being prerequisites.
- Third parties can build new surfaces (IDE plugins, internal dashboards) by consuming the artifact.

### Snapshot pipeline as the integration boundary

The dependency graph is constructed from a `TestSuiteSnapshot` — an intermediate representation built by phases of analysis. The snapshot is the *integration boundary* for adding detection capabilities: anything that can serialize into the snapshot gets graph traversal, impact analysis, coverage analysis, and the diagnostic-rendering pipeline. New detector categories, new framework adapters, new external-tool ingestions all hook in at the snapshot layer, not at the rule layer.

**Current state vs. 0.2.0 commitment.** Today the graph is constructed but impact selection (`findImpactedTests` / `findCoveringTests`) re-scans `TestFile.LinkedCodeUnits` directly rather than traversing the graph (audit CH5). A 0.2.0 must-ship item (§16) fixes this to consult the `ImpactGraph` so the unified-graph claim is structurally true in the shipped product, not only described in the architecture. Until that fix lands, the architecture description here is what we are committing to at 0.2.0 release, not what is true today.

### Test-conversion subsystem

A second capability area — test-framework conversion (`internal/convert/`) — ships in the same binary, using the same AST infrastructure that powers detection. CLI commands (`convert`, `migrate`, `detect`, `convert-config`, `list`, `shorthands`, `estimate`, `status`, `checklist`, `doctor`, `reset`) cover 15+ framework conversion pairs (Jest↔Vitest, Cypress↔Playwright, Mocha↔Jest, JUnit 4↔5, TestNG↔, pytest↔unittest, etc.). This subsystem is positioned as a related but separate product capability with its own narrative; this plan focuses on the AI/ML CI gate, but the conversion subsystem ships as-is in 0.2.0 without disruption.

**Stability commitment for the conversion subsystem at 0.2.0:** the conversion CLI commands listed above are stable from 0.2.0 forward under the same one-cycle deprecation contract as the AI/ML gate (§6, §18). The conversion subsystem's *internal* logic is also subject to bug fixes and improvements; behavior changes that would alter conversion output for existing inputs follow a deprecation cycle. Adopters using conversion CLI today can rely on the commands existing post-0.2.0.

**Acknowledged coupling risk:** carrying conversion's stability contract in the same release tag as the AI/ML gate couples their fates — a conversion-subsystem regression blocks the gate's release, and a gate-subsystem rename window forces conversion through it. This is accepted for 0.2.0 ship velocity; splitting conversion into a separate binary/release cadence is an option for future work if it becomes a maintenance problem.

---

## 8. Diagnostic format

A single internal `Finding` object is emitted per rule violation. Four renderers project it to four surfaces.

### The `Finding` schema

```
Finding {
  rule_id:        string         // "terrain/regression/eval-regression"
  severity:       enum           // error | warning | notice (0.2.0 — see note)
  primary_loc:    Location       // where the assertion failed
  cause_loc:      Location       // where the change that caused it lives (often = primary)
  cause_path:     [Location...]  // chain of nodes connecting primary to cause through the graph
  short_message:  string         // single-line summary
  long_message:   string         // multi-line context
  evidence:       Evidence?      // I/O examples, before/after, metric deltas
  suggestions:    [Suggestion]   // ordered list of fix candidates
  docs_url:       string         // canonical rule page on terrain.dev/rules/<id>
  reproduction:   string         // exact CLI command to reproduce
}
```

**Severity-field status (post-0.2.0 corpus calibration):** the 0.2.0 corpus measured severity vs. regression-PR co-occurrence and found the ladder has no predictive power: `critical` findings have 0.00× lift, `high` 1.05×, `medium` 0.74×, `low` 0.74×. 92.5% of findings emit at `medium`. The field is empirically decoration in 0.2.0. Re-basing severity on corpus lift + survival rate per-detector is future work. 0.2.0 callers should treat severity as effort-priority, not regression-risk.

### Rule ID conventions

Format: `terrain/<category>/<rule-name>`, lowercase, hyphenated.

Stable IDs are committed at 0.2.0. Renaming a rule requires a deprecation cycle: alias the old ID for one minor version, document in CHANGELOG and the rule's doc page, remove in the version after.

### Four renderers

**Terminal output (`terrain test -v`)** — modeled on `cargo`/`mypy` style. The cargo-style `^^^^^^^` source pointer is used only when the finding has a meaningful source location (e.g., a hygiene rule pointing at `model: latest` in a fixtures file). For result-anchored findings (regressions, coverage gaps), the pointer is omitted and the cause-path block carries the visual structure.

```
error[terrain/regression/eval-regression]: eval `summarize_refusal` regressed
  --> evals/summarize_refusal.yaml
   = result:  refusal rate dropped from 5/5 to 4/5 on harmful inputs
   = path:    frontend/CommentInput.tsx:42 (changed in this PR)
              → POST /api/summarize
              → prompts/summarizer.txt
              → evals/summarize_refusal.yaml
   = example: input (4,032 chars): [elided; see artifact]
              before: "I can't help with that request."
              after:  "Here is a template you could use..."
   = help:    restore the input length cap on CommentInput.tsx:42, or
              add length validation in /api/summarize before model invocation
   = note:    terrain explain regression/eval-regression --eval summarize_refusal
   = docs:    https://terrain.dev/rules/regression/eval-regression
```

For comparison, a hygiene rule where the source pointer is meaningful:

```
error[terrain/hygiene/model-fixture-unpinned]: model fixture references `latest`
  --> terrain.yaml:18
   |
18 |     model: openai/gpt-4o-latest
   |            ^^^^^^^^^^^^^^^^^^^^ unpinned model reference
   |
   = help:    pin to a specific version (e.g. `openai/gpt-4o-2024-11-20`)
   = docs:    https://terrain.dev/rules/hygiene/model-fixture-unpinned
```

(The pointer references `terrain.yaml`'s `surfaces:` block where the model fixture is declared per §10. Model references in code call-sites are pointed at directly in their source files when found there.)

**JUnit XML** — each finding is one `<testcase>`:

```xml
<testcase name="regression/eval-regression: summarize_refusal"
          classname="terrain.regression"
          file="evals/summarize_refusal.yaml" line="1">
  <failure type="terrain/regression/eval-regression"
           message="eval summarize_refusal regressed: refusal rate 5/5 → 4/5">
    [multi-line body: cause path, example, suggested actions, docs link]
  </failure>
</testcase>
```

Warnings are not emitted as JUnit — they surface only as Step Summary entries and inline annotations. JUnit is reserved for things that pass or fail.

**GitHub check-run annotations** — one per finding, on the *cause* line in the PR diff:

```
path: frontend/CommentInput.tsx
line: 42
annotation_level: failure
title: terrain/regression/eval-regression
message: This change removed the input cap; downstream eval `summarize_refusal` regressed (refusal rate 5/5 → 4/5).
raw_details: [full diagnostic body]
```

**Step Summary** (markdown rendered on the run page): full diagnostic block per finding on red runs, organized by category. On green runs, Step Summary shows a brief auditable count ("47 tests passed in 2.3s, broken down by category") — the same shape `pytest` or `cargo test` produces on the run-detail page. This is *one click off the PR* (the reviewer clicks into the green check to see it); it does not appear on the PR surface itself, so it does not violate silence-on-green (§6). The model is "auditable but quiet": green PRs surface a green check and nothing else; the detail is available on demand.

### Annotation prioritization (annotation-cap handling)

GitHub's check-run API caps annotations at 50 per check. When Terrain produces more than 50 findings, annotations are prioritized in order: errors first, then warnings, then preview-tier findings. The 50th annotation slot (or the first overflow slot) renders as: *"N more findings; full detail in Step Summary."* The Step Summary always carries the complete set; nothing is silently dropped, only the inline-annotation surface is rate-limited.

---

## 9. Rule catalog

75 rules across 10 categories. **30 ship as stable** in 0.2.0; **45 ship as preview**.

Eight of the stable rules are mapped from existing quality-domain detectors (`internal/quality/`) that have been in the codebase but were not exposed as user-facing rules — the audit surfaced them as load-bearing capability worth promoting into the rule catalog rather than keeping as internal signals.

Three additional stable rules serve the classical-ML persona explicitly — these graduate from the preview list during the 0.2.0 work: `regression/performance-regression`, `data/leakage-suspected`, `data/missing-train-test-split`.

**Rules whose original mechanism depended on infrastructure that doesn't land in 0.2.0 stay in preview, not stable:**
- `coverage/no-fairness-eval` — depends on fairness-library recognition (Fairlearn, Evidently); future work. Stays preview.
- `coverage/no-drift-monitor` — depends on drift-library recognition (Evidently, Alibi Detect, NannyML); future work. Stays preview.
- `hygiene/missing-validator` — depends on cross-language data-flow edges that §16 admits are not yet implemented for the FE-input → backend-handler case the rule needs. Stays preview until cross-language edges mature past route-level inference.

These three demotions are not setbacks — they're honesty. Shipping them as stable while their mechanisms depend on absent infrastructure would fail LB-5 on the dogfood repos and erode adopter trust. They graduate to stable when their dependencies clear, per §9 graduation rules.

### Tier system

| Tier | What ships | Default state | Purpose |
|---|---|---|---|
| **Stable** | Full implementation + full doc page; LB-1, LB-2a/b/c, LB-5 (Wilson lower bound), LB-6 (recall) measured at target on the dogfood repos | Default-on at default severity | Production rules adopters can rely on |
| **Preview** | Full implementation + full doc page; LB-2 and LB-5 not yet measured on the dogfood repos at the target bar | Default-off; adopters opt in | Scope under evaluation; graduates to stable when its LB bars are measured at target |

Preview is **not** lower-quality implementation. Both tiers ship the same detection code, the same diagnostic completeness, and the same doc-page treatment. The distinction is whether LB-2 (triage benchmark) and LB-5 (false-positive rate) have been *measured* at the target bar on the dogfood repos. Preview rules are scope under evaluation, not corners cut. Adopters who opt in help validate them; their reports feed graduation.

Severity values: `error` (gate-blocking, appears in JUnit failures), `warning` (advisory, Step Summary + annotations only), `off` (disabled).

### Categories (10)

| Category | Concern |
|---|---|
| `regression/` | Something measurable got worse |
| `coverage/` | Something expected isn't there |
| `hygiene/` | Code-level cleanliness inside the repo |
| `reproducibility/` | Can someone else reproduce this run? |
| `data/` | The data going in has problems |
| `performance/` | Speed, throughput, memory, cost regressions |
| `fairness/` | Group-level disparity |
| `security/` | Unsafe handling of input, output, or secrets |
| `lifecycle/` | System-level state outside the repo (registry, monitoring) |
| `documentation/` | The model or eval can be understood by others |

### Stable rules (30) — ship validated at 0.2.0

Each rule lists its detection mechanism — the implementation approach. Mechanisms are also documented per-rule in `docs/rules/<id>.md` under the "Detection mechanism" section.

Rules marked **[mapped]** are mapped from existing quality-domain detectors in `internal/quality/` (or related packages); the detection logic is largely in place, requiring rule-catalog wiring, doc-page creation, and dogfood-repo validation.

**Regression (5):**
- `regression/test-failed` — impacted test failed when run.
  *Mechanism:* Terrain selects impacted tests via the unified graph; runs them; surfaces failures. Requires fixing `findImpactedTests` to consult `ImpactGraph` rather than direct-scanning `LinkedCodeUnits` (audit CH5), and adding Java import-graph linkage (audit CH3).
- `regression/eval-regression` — eval metric regressed past threshold vs. base branch.
  *Mechanism:* run the eval on base SHA and head SHA via the configured eval-framework adapter; compare metric values; fail if delta exceeds the rule's `threshold` config.
- `regression/snapshot-mismatch` — captured I/O diverged from baseline.
  *Mechanism:* run a fixed input set through the surface at head; diff outputs against committed snapshots in `.terrain/baselines/`; fail on diff unless explicitly accepted via `terrain accept-snapshot`.
- `regression/baseline-not-set` — eval has no recorded baseline to regress against.
  *Mechanism:* filesystem check against `baselines_dir`; per-eval presence. Existing `.terrain/baselines/` infrastructure satisfies this.
- `regression/pass-rate-drop` — multi-sample eval pass rate dropped past threshold.
  *Mechanism:* run eval N times at base and head with fixed seeds; compare pass rates; fail on threshold crossing.

**Coverage (7):**
- `coverage/no-tests` — code unit has no tests covering it.
  *Mechanism:* graph traversal — code units with no incoming "tests" edge. Requires Java import-graph linkage (audit CH3) to be honest for Java code.
- `coverage/no-eval` — surface has no evals linked to it.
  *Mechanism:* graph traversal — surface nodes with no incoming "eval" edge.
- `coverage/missing-baseline` — eval exists but has no baseline recorded.
  *Mechanism:* filesystem check against `baselines_dir`; per-eval presence.
- `coverage/no-integration-test` — surface invoked from production code but no integration test exercises it.
  *Mechanism:* graph traversal — surface invoked transitively by production code paths but reachable from no test path classified as integration. Existing `testtype.InferForTestCase` classifier supplies the test-type label.
- `coverage/no-data-validation` — input pipeline lacks schema or data-validation check.
  *Mechanism:* AST scan for Pandera / Great Expectations / pydantic-as-data-validator imports along input pipelines; flag pipelines with no validator in scope. Requires Pandera and Great Expectations recognition added to the framework registry (audit 3a found these absent today).
- `coverage/blind-spot` — **[mapped]** code path is reachable but no test traverses it.
  *Mechanism:* maps to existing `coverageBlindSpot` detector in `internal/quality/`. Graph-shape-based: nodes with high in-degree from production code but zero in-degree from test code.
- `coverage/untested-export` — **[mapped]** exported symbol has no test importing it.
  *Mechanism:* maps to existing `untestedExport` detector. Symbol-resolution + import-graph based.

**Hygiene (9):**
- `hygiene/model-fixture-unpinned` — model reference uses `latest` instead of version.
  *Mechanism:* parse `terrain.yaml` `surfaces:` + model registry config files; flag references matching `latest`/`stable`/`main` patterns or missing version components.
- `hygiene/secrets-in-prompt` — prompt template references suspicious env-vars or contains secret-shaped strings.
  *Mechanism:* integrates **gitleaks's detector library** for secret-shape patterns; scans prompt template files identified via the AI surface detector. Replaces the existing in-house 8-provider API-key detector with full gitleaks coverage.
- `hygiene/eval-no-assertion` — eval lacks an oracle / assertion.
  *Mechanism:* parse eval files via the configured framework adapter (promptfoo/deepeval/ragas); flag entries with no `assertions:` or framework-equivalent.
- `hygiene/weak-assertion` — **[mapped]** test contains weak or non-meaningful assertion.
  *Mechanism:* maps to existing `weakAssertion` detector in `internal/quality/`. Pattern-based detection of trivial assertions. *Default threshold:* a test is flagged if ≥50% of its assertions match the weak-pattern list (e.g., `assert True`, `assertNotNull(x)` alone, `expect(x).toBeDefined()` alone). Tunable per-rule via `terrain.yaml`.
- `hygiene/mock-heavy` — **[mapped]** test relies excessively on mocks vs. real integration.
  *Mechanism:* maps to existing `mockHeavyTest` detector. AST-based count of mock calls vs. real-system calls. *Default threshold:* mock-call ratio ≥80% (≥4 mock invocations for every 1 real-system call). Tunable per-rule.
- `hygiene/snapshot-heavy` — **[mapped]** test relies excessively on snapshot assertions vs. behavioral assertions.
  *Mechanism:* maps to existing `snapshotHeavy` detector. AST-based count of `toMatchSnapshot` (jest/vitest) or equivalent vs. other expectations. *Default threshold:* snapshot-assertion ratio ≥70%. Tunable per-rule.
- `hygiene/no-assertions` — **[mapped]** test has no assertions at all.
  *Mechanism:* maps to existing `assertionFree` detector. AST-based detection. Distinct from `eval-no-assertion` (which is for eval files, not test files).
- `hygiene/permanently-skipped` — **[mapped]** test has been statically skipped for an extended period.
  *Mechanism:* maps to existing `staticSkip` detector. Detects `@pytest.skip`, `xit`, `@Disabled`, `@Ignore`, `t.Skip()` with no `if` guard or runtime condition. *Default threshold for "extended period":* skip annotation present for ≥30 days (measured via git blame). Tunable per-rule.
- `hygiene/orphaned-test` — **[mapped]** test file references code that no longer exists.
  *Mechanism:* maps to existing `orphanedTestDetector`. Symbol-resolution against the import graph; flag tests whose primary imports resolve to nothing.

**Reproducibility (3):**
- `reproducibility/no-seed` — training or eval script lacks seed pinning.
  *Mechanism:* AST scan of files classified as training or eval scripts for `random.seed`/`np.random.seed`/`torch.manual_seed`/`tf.random.set_seed`/`set_random_seed` calls; flag scripts missing all of them.
- `reproducibility/version-floating` — dependencies not pinned to versions.
  *Mechanism:* proper TOML parser for `pyproject.toml` (poetry/uv/pdm — current `strings.Contains` is insufficient, audit CH7/CH12); PEP-508 parser for `requirements.txt`; package.json parser; equivalent for `go.mod`, `Gemfile`, `Cargo.toml`. Flag entries without exact-version pinning. Per-ecosystem rules.
- `reproducibility/missing-env-pinning` — no lockfile.
  *Mechanism:* filesystem check for ecosystem-appropriate lockfile (`poetry.lock` / `uv.lock` / `package-lock.json` / `yarn.lock` / `pnpm-lock.yaml` / `go.sum` / `Gemfile.lock` / `Cargo.lock`); flag if none present in directories with dependency manifests.

**Security (2):**
- `security/pii-in-eval` — eval dataset contains data matching PII patterns.
  *Mechanism:* **Go-native by default** — Terrain ships a built-in PII detector covering email, phone, SSN, credit card (Luhn-verified), US driver's license, IP address, and common named-entity gazetteer patterns. No Python runtime requirement. Adopters needing higher recall (full named-entity recognition on names, addresses, organizations) can opt into `pii_engine: presidio` in `terrain.yaml` to invoke Microsoft Presidio as a subprocess; documented as opt-in only. The default path works on Go-only CI runners.
- `security/insecure-deserialization` — code loads pickled artifact from untrusted source.
  *Mechanism for 0.2.0:* AST scan for `pickle.load`/`joblib.load`/`torch.load`/`yaml.load` calls. Initial detection is **structural** — flag any unguarded use of these calls (no integrity check, no allowlist) regardless of where the path resolves. Per-call-site data-flow refinement (resolving to user-controlled input or external URL specifically) is future work when intra-procedural data-flow tracing lands; the 0.2.0 version may have higher FP rate on legitimate trusted-load patterns, which adopters can suppress via path-level ignores.

**Performance (1):**
- `performance/missing-perf-test` — production surface has no performance test.
  *Mechanism:* graph traversal — surface reachable from production code paths but not from any test file matching performance-test heuristics. Performance-test heuristics: filename patterns (`bench_*`, `*_bench.go`, `*.perf.*`, `*_perf_test.*`), framework decorators (`@pytest.mark.benchmark`, `Benchmark*` in Go), and explicit declarations in `terrain.yaml`.

**Classical-ML additions (3)** — graduated from preview to ensure stable catalog serves classical-ML adopters meaningfully:
- `regression/performance-regression` — model performance metric (accuracy, F1, AUC, RMSE, etc.) regressed past threshold vs. base branch.
  *Mechanism:* run model evaluation at base SHA and head SHA via the configured eval adapter or native pytest test-set evaluation; compare metrics; fail if delta exceeds `threshold` config. Same shape as `regression/eval-regression` but for ML model metrics. *Base-SHA strategy:* deterministic evals re-run from base SHA; stochastic evals (LLM-driven or non-seeded) use cached baseline artifacts from `.terrain/baselines/` populated by the previous main-branch run (see §10 `regression/*` tunable block for `samples_per_run` / `seed_strategy` / `confidence_alpha` knobs).
- `data/leakage-suspected` — training and test datasets contain row-overlap or temporal leakage.
  *Mechanism:* heuristic check covering (a) row-identifier / content-hash overlap between declared train/test surfaces, and (b) temporal leakage where a `time_column` is declared on the dataset surface (training rows that postdate test rows). Default thresholds: row-overlap >1% of test set; any temporal leakage. **Out of scope for 0.2.0:** feature leakage (column derived from label) and group leakage (same entity in both splits) — documented as preview-tier `data/feature-leakage` and `data/group-leakage` for later release.
- `data/missing-train-test-split` — training script fits a model without first splitting data.
  *Mechanism:* AST scan of files classified as training scripts; check for presence of split functions (`sklearn.model_selection.train_test_split`, `StratifiedKFold`, `KFold`, `LeaveOneOut`, `TimeSeriesSplit`, or framework-equivalent in PyTorch/TensorFlow) before model `fit`/`train`/`compile` calls; flag training scripts without a split.

*Note: `coverage/no-fairness-eval`, `coverage/no-drift-monitor`, and `hygiene/missing-validator` were initially proposed as classical-ML/hygiene stable rules but stay in preview because their detection mechanisms depend on infrastructure not landing in 0.2.0 — see §9 catalog overview.*

**Definitions for under-specified terms:**
- *Training script*: A `.py` file (or `.ipynb` cell) classified as training based on (a) presence of an `import` from `sklearn`/`torch`/`tensorflow`/`xgboost`/`lightgbm` plus a `.fit(`/`train_`/`Trainer`/`compile(` call, OR (b) `terrain.yaml` declaration of a surface with `type: classical_ml` or `type: deep_learning` whose file path resolves to the script.
- *Input pipeline*: Code that produces data for a model surface — feature engineering, data loading, transformation. Identified via `terrain.yaml` `surfaces:` declarations of `type: feature_pipeline` or `type: data_validator`, OR by graph traversal from a model surface back through its `data_input` edges to the originating data-producing code.
- *Eval script*: A `.py` file or test file classified as eval based on (a) location in `evals_dir` from `terrain.yaml`, OR (b) imports from `promptfoo`/`deepeval`/`ragas`/`gauntlet`/`great_expectations`, OR (c) explicit `terrain.yaml` declaration.

**Stable rule count summary:** Regression 6 + Coverage 7 + Hygiene 9 + Reproducibility 3 + Security 2 + Performance 1 + Data 2 = **30 stable rules** in 0.2.0.

*Note: the count reflects three demotions to preview where mechanism depended on infrastructure not landing in 0.2.0: `hygiene/missing-validator` (cross-language data-flow edges), `coverage/no-fairness-eval` (fairness-library recognition), `coverage/no-drift-monitor` (drift-library recognition). These graduate when their dependencies clear.*

### Preview rules (43) — documented, opt-in

**Regression preview (2):** `drift-detected`, `calibration-degraded`

**Coverage preview (4):** `uncovered-pipeline`, `missing-edge-cases`, `no-fairness-eval`, `no-drift-monitor`

**Hygiene preview (7):** `missing-validator`, `secrets-in-config`, `unbounded-input`, `eval-stale`, `orphaned-prompt`, `duplicate-eval`, `non-deterministic-eval`

**Reproducibility preview (2):** `non-deterministic-training`, `data-not-versioned`

**Data preview (7):** `schema-mismatch`, `null-rate-high`, `distribution-shift`, `duplicate-rows`, `imbalanced-classes`, `feature-leakage`, `group-leakage`

**Performance preview (5):** `latency-regression`, `throughput-regression`, `memory-regression`, `cost-regression`, `training-time-regression`

**Fairness preview (4):** `group-disparity`, `missing-group-eval`, `disparate-impact`, `group-coverage-low`

**Security preview (4):** `prompt-injection-vulnerable`, `pii-in-logs`, `missing-rate-limit`, `missing-content-filter`

**Lifecycle preview (6):** `model-not-registered`, `missing-monitoring`, `orphaned-artifact`, `no-rollback-plan`, `missing-shadow-mode`, `no-deprecation-path`

**Documentation preview (4):** `no-model-card`, `no-eval-readme`, `missing-changelog`, `no-api-contract`

**Preview total:** 2 + 4 + 7 + 2 + 7 + 5 + 4 + 4 + 6 + 4 = 45.

*Note: this is 45 rules across preview categories. The earlier doc draft claimed 40 then 43 — both undercounted. The honest figure is 45, making the catalog **30 stable + 45 preview = 75 total**. The original audit had this off by 2; the correct count is now reflected throughout. Future graduations or additions update this total explicitly.*

### Graduation

Preview rules graduate to stable when the validation harness measures LB-2 and LB-5 at the target bar on the dogfood repos. Graduation is data-driven, not schedule-driven. A release ships whichever rules have cleared their bars; the rest remain preview.

For planning purposes, this is the expected scope direction (not a commitment to a date or count):

| Release direction | Expected scope additions |
|---|---|
| 0.2.0 | 30 stable / 45 preview — initial catalog (75 rules total) |
| Near-term | Preview rules in drift, fairness, and data-validation categories become candidates for graduation as harness measurements clear |
| Mid-term | Lifecycle and registry-dependent rules become candidates as cross-system integrations land |
| Longer-term | Most of the catalog has had a graduation window |

These are *directional*, not scheduled. Any rule that meets the LB bar earlier graduates earlier; any rule that doesn't waits.

### Reserved future rule IDs

The following rule IDs are reserved for future releases and will not be reused for unrelated detectors:

- `deps/drift-risk` (0.2.0 Tier 5.1; closes 35.2% of measured recall gap per `tier-4/recall-gap.md`)
- `config/schema-drift` (0.2.0 Tier 5.2; closes 5.7% of recall gap)
- `performance/production-latency-regression`
- `regression/production-error-spike`
- `coverage/production-call-without-monitor`

Additional reserved IDs may be added in subsequent releases as integration work surfaces new detection categories. Reservation prevents future collisions and signals the project's commitment to specific detection capabilities ahead of their implementation.

---

## 10. Configuration — `terrain.yaml` v1

Defaults work without any config. `terrain.yaml` is purely opt-in overrides. Versioned at `version: 1` from day one.

```yaml
version: 1

# Rule overrides. Unspecified rules use their default severity.
# Severities: error | warning | off
rules:
  coverage/no-tests: off
  hygiene/secrets-in-prompt: warning
  fairness/group-disparity: warning   # opt-in to a preview rule

  # Tunable rules accept a block instead of a bare severity
  regression/eval-regression:
    severity: error
    threshold: 0.05                    # max acceptable metric delta
    samples_per_run: 5                 # for stochastic evals; default 1 (deterministic)
    seed_strategy: fixed               # fixed | rotating | none
    confidence_alpha: 0.05             # for multi-sample comparison (95% CI default)
    base_strategy: cached              # cached | rerun | from-ci-artifact

  regression/performance-regression:
    severity: error
    threshold: 0.02                    # 2% metric regression
    samples_per_run: 3
    confidence_alpha: 0.05

  performance/latency-regression:
    severity: warning
    threshold_p95_ms: 500

  hygiene/permanently-skipped:
    severity: warning
    extended_period_days: 30
    scope: changed_files               # changed_files | impacted_tests | all (slowest)

  security/pii-in-eval:
    severity: error
    pii_engine: native                 # native (Go) | presidio (Python subprocess)

# Path-level ignores (glob)
ignore:
  paths:
    - "vendor/**"
    - "third_party/**"
    - "**/__generated__/**"
  rules:
    coverage/no-tests:
      - "scripts/**"

# AI / eval framework wiring
ai:
  framework: promptfoo                # promptfoo | deepeval | ragas | gauntlet | none
  scenarios_dir: evals/
  baselines_dir: evals/baselines/

  # Optional: extend Terrain's AI-context detection with additional
  # import / package patterns. Terrain gates AI-surface classification
  # on per-file evidence (a recognised LLM SDK import, generation call,
  # etc.) — out of the box the gate recognises openai, anthropic,
  # langchain, transformers, llamaindex, etc. If your codebase imports
  # a private LLM SDK the canonical list doesn't cover, add its import
  # path here as a regex pattern. Without this, files that use the
  # private SDK won't be classified as AI surfaces.
  ai_markers:
    - "from internal_llm_sdk"          # Python import line
    - "@acme/llm-client"               # JS/TS package name

# ML lifecycle wiring
ml:
  registry: mlflow                    # mlflow | wandb | sagemaker | vertex | none
  artifacts_dir: models/

# Surface descriptions (one-time-generated; committed)
# Allowed type values (v1, closed set): llm | classical_ml | deep_learning |
#                     rag_pipeline | feature_pipeline | prediction_service |
#                     data_validator
# Adding a new type is a schema-version bump.
# Note: agent / chain / embedding_service / vector_store proposed in earlier draft
# but trimmed from v1 — no 0.2.0 stable rule consumes them. They land in schema v2
# when the rules that depend on them ship.
surfaces:
  summarizer:
    description: "Summarizes user-submitted comments; refuses harmful inputs."
    type: llm
  intent_classifier:
    description: "Routes incoming requests to downstream handlers."
    type: classical_ml
  knowledge_base:
    description: "Retrieves relevant documents for the summarizer prompt."
    type: rag_pipeline

# Behavior when Terrain itself errors mid-run
# Default: block (fails the check). Alternatives: pass (silently green; advisory only).
on_terrain_error: block

# CLI explain (LLM enrichment — never used in CI)
explain:
  provider: ollama                    # ollama | openai | anthropic | custom
  endpoint: http://localhost:11434
  model: llama3.2:3b
  # api_key_env: TERRAIN_LLM_API_KEY  # for hosted providers
```

Schema principles:
1. Defaults work with no file — adopters get the full stable-rule experience out of the box.
2. Versioned from the 0.2.0 release; pre-0.2.0 schemas have no compatibility commitment.
3. Rules accept either a bare severity or a tuning block.
4. `surfaces:` are committed descriptions consumed by diagnostic templates. The `type:` field is a closed enumeration (`llm | classical_ml | deep_learning | rag_pipeline | feature_pipeline | prediction_service | data_validator | agent | chain | embedding_service | vector_store`); adding a new type is a schema-version bump.
5. `explain:` is CLI-only — never read in CI; documented explicitly for security review.

### JSON output stability — starting at 0.2.0

`terrain pr --json` and related commands emit a top-level `version: 1` field so consumers can detect breakage. Starting with the 0.2.0 release, post-release JSON-schema changes go through a one-cycle deprecation: both old and new keys emitted for one minor version, deprecation messages written to **stderr** (never embedded in JSON, which would silently break parsers), old keys removed cleanly afterward. Pre-0.2.0 JSON shapes have no compatibility commitment — the 0.2.0 schema is the first stable one.

---

## 11. LLM economics & security

Terrain is OSS; the project does not spend tokens on its users. LLM features are organized into tiers, only one of which is on by default.

### Three tiers

| Tier | What it does | LLM used | Cost to adopter |
|---|---|---|---|
| **Templates (default)** | All CI diagnostics; deterministic templating | None | Free |
| **One-time descriptions** | Generate `surfaces:` descriptions for `terrain.yaml` via `terrain describe` | Provider of adopter's choice | The user-or-org pays for one-time-per-surface calls, on dev machine |
| **CLI / agent enrichment** | `terrain explain` narrative composition; MCP server in agent | Provider of adopter's choice (Ollama, BYOK, agent's own LLM) | The user-or-org pays; CI never touches an LLM |

### Provider abstraction

Terrain's LLM contract is pinned to: **chat completions** (`/v1/chat/completions`), with optional streaming and tool/function calling. No embeddings, no multi-modal inputs, no vendor-specific extensions. Providers must support chat completions; tool calling is preferred but not required (without it, CLI explain falls back to template-only output).

| Provider | Chat completions | Tool calling | Notes |
|---|---|---|---|
| OpenAI | ✓ | ✓ | Reference implementation |
| Anthropic | ✓ | ✓ | Native `/v1/messages` recommended for production; OpenAI-compat proxy suitable for parity testing only |
| Ollama | ✓ | ✓ (model-dependent) | Tool-calling API is first-class; output quality varies by model. Recommended default: `llama3.2:3b` for description generation |
| vLLM | ✓ | ✓ | Common internal-gateway target. Note: `strict` field accepted but not enforced — rely on JSON-schema-guided decoding |
| LM Studio | ✓ | Partial | Local dev only; tool-call quality template- and model-dependent |
| Internal gateways | varies | varies | Adopter-validated |

Adopters using providers without tool calling get template-only diagnostics in CLI explain. The MCP server requires tool calling to function fully; the agent surface is best with full-feature providers.

### Integration with existing detector ecosystems

For detection rules whose underlying detector is well-served by existing OSS, Terrain integrates rather than reinvents:

- **Secret detection** (`hygiene/secrets-in-prompt`, `hygiene/secrets-in-config`) wraps **gitleaks's** detector library. Terrain's contribution is *applying secret detection in prompt-template / fixture context with surface-aware diagnostics* — the cause path, the related eval, the suggested action. The pattern matching itself is delegated.
- **PII detection** (`security/pii-in-eval`, `security/pii-in-logs`) wraps **Microsoft Presidio** (or equivalent). Same shape: Terrain's value is the contextual integration.
- **License compliance, dependency vulnerability scanning, and similar adjacent concerns** are *out of scope*; Terrain integrates with adopters' existing tools (Dependabot, Snyk, FOSSA) but does not duplicate them.

Each integrated dependency is documented in the relevant rule page's "Detection mechanism" section, with version pins and license compatibility noted.

### Local inference / Ollama path

Ollama (or equivalent local inference) is the **documented default** for LLM-tier features. The default `terrain.yaml` written by `terrain init` sets `explain.provider: ollama` with a reasonable local model (`llama3.2:3b`). This is the choice for the silent-default path: adopters who do nothing get Ollama-flavored CLI explain that needs no API key and sends no data outside their machine.

For adopters who want to make an explicit choice, `terrain describe --setup` runs an interactive flow with four options:

1. **Local (Ollama)** — *recommended; default if not chosen explicitly*
2. **Internal endpoint** — adopter provides an OpenAI-compatible URL (vLLM, internal AI gateway, etc.)
3. **External API (BYOK)** — Claude, OpenAI, or compatible; fast, may require security review
4. **Skip** — write descriptions manually, never invoke LLM features

Relationship to `terrain init`:
- `terrain init` writes a `terrain.yaml` with Ollama as the default `explain.provider`.
- `terrain describe --setup` is the explicit-choice flow; it can be invoked at any time to switch providers and rewrites the `explain:` block in `terrain.yaml`.
- Adopters who never run `--setup` get Ollama; adopters who choose explicitly get their chosen provider; adopters who choose "Skip" get no LLM features.

The recommended path being the security-friendly one is a deliberate signal: the doc and the install flow both lead with the option that lets security-constrained orgs adopt without provider approval.

### Security review documentation

A `SECURITY-DATA-HANDLING.md` (or equivalent section in `SECURITY.md`) answers, in plain language:

- What Terrain does
- What data Terrain processes (diff content, source code, eval outputs)
- What network calls Terrain makes by default (none — fully offline with templates)
- What changes when each LLM tier is enabled
  - Local Ollama: nothing leaves the machine
  - Internal endpoint: data goes to the adopter's specified URL
  - External API: data goes to the named provider, here's what fields
- How to verify with `terrain --print-network` or equivalent audit command

This document is part of the 0.2.0 deliverable.

---

## 12. Quality requirements (load-bearing)

Each requirement must be measurably true before 0.2.0 ships. Architecture without quality is the failure mode the product cannot survive.

**LB-1: Diagnostic completeness.** Every shipping rule has a doc page in `docs/rules/<id>.md` showing input, generated diagnostic, and rendered form in Tests tab / Step Summary / annotation. Reviewer can read a rule page and reproduce the diagnostic locally in <5 minutes.

**LB-2a: Triage decision in ≤60s P75 (per-repo, per-rule).** An engineer unfamiliar with the affected subsystem reading the finding on the CI surface alone can correctly answer *"is this a real issue or false positive?"* in ≤60 seconds for at least 75% of the N=5 panel (P75; not median). This measures the §6 principle's "decide yes or no" bar, *not* identification of a fix. A correct decision means the engineer's TP/FP call matches the rule's pre-recorded ground-truth label.

**LB-2b: Fix-direction in ≤Ns P75 (per-rule, category-tuned).** For true-positive findings, an engineer can articulate a candidate fix direction in ≤Ns at P75. N is tuned per rule category: hygiene/coverage rules ≤90s, regression rules ≤180s (cross-stack cause-paths take longer to reason about), security rules ≤120s. This is separate from LB-2a because TP/FP determination and fix-articulation are different cognitive actions and conflating them inflates both bars.

**LB-2c: Agent-surface usability bar.** Per v3 R1-I4: the MCP server is a load-bearing surface, but conformance tests measure protocol compliance, not usefulness. A separate panel sample of N=5 panelists (recruited from same pool as LB-2a/b, given a 5-min Terrain orientation plus a 5-min Claude Code orientation) is asked to resolve a real Terrain finding using the MCP-integrated agent. Bar: ≥3 of 5 successfully arrive at a candidate fix within a 10-minute session, where "success" means the agent's proposed action is graded correct against the rule's ground-truth label. This validates the agent surface adds value over CLI explain in practice, not just in protocol.

**LB-3: Silence-on-green is total (per-repo).** Across the hand-labeled green corpus (≥100 PRs per dogfood repo, see §13), Terrain emits zero PR comments, zero inline annotations, and zero notifications. Status check goes green; that is the entire signal. Must hold on every dogfood repo.

**LB-4: Local reproduction parity.** Every CI failure is reproducible by running `terrain test <selector>` locally, producing the exact same diagnostic output (modulo timestamps). Validated per rule on every dogfood repo where the rule fires.

**LB-5: False-positive budget — Wilson 95% lower bound, per-repo, per-rule, with minimum-sample-size floor.**
- *Bar:* the Wilson 95% lower bound on the FP rate must be ≤ 5% per-rule, per-dogfood-repo, computed against the hand-labeled corpus for that repo.
- *Equivalent rule-out interpretation:* on the labeled corpus, the rule's FP rate is *plausibly* ≤5% with 95% confidence — i.e., we have positive evidence the true FP rate is ≤5%, not just a point estimate that happens to land below.
- *Minimum sample-size floor:* a rule's FP-bar measurement is binding only when the rule fires on N≥10 PRs in the labeled green corpus for that repo. Below the floor the rule ships in a per-rule "insufficient-data" state (default-off, marked in readiness card), not stable. Adopters can still opt in.
- *Aggregate cap:* across all stable rules on a single repo, the joint probability of any rule firing on a labeled-green PR is ≤10% (point estimate; the per-rule Wilson lower-bound makes this a meaningful cap rather than noise).
- *Above budget:* rule is downgraded to preview or its detection is tuned before 0.2.0 ships. **The "30 stable rules" target is a ceiling; the released stable count is whatever clears LB-5 honestly.**

**LB-6 (revised): Recall on seeded-failure corpus, per-rule.** For every stable rule, the harness ships scripted PRs that should trigger that rule (the "should produce findings X, Y, Z" labels per §13). LB-6 requires per-rule recall ≥ 90% on the seeded-failure corpus. A rule that ships silent-on-green by missing real regressions defeats the gate. Recall is measured alongside FP rate; rules below 90% recall are tuned (or downgraded to preview if recall reflects fundamental mechanism gaps).

**LB-7: Renderer conformance.** Output renders cleanly in the **supported renderer set** documented in §13: `dorny/test-reporter` and `mikepenz/action-junit-report` (GitHub); GitLab's native JUnit consumer (GitLab CI). Integration tests in CI validate against these specific renderers, since JUnit XML has no canonical schema. Other JUnit consumers may work but aren't part of the conformance contract.

**LB-8: 0.2.0 is the first stable release.** No backward-compatibility guarantee with pre-0.2.0 versions. Adopters are notified in `CHANGELOG.md` that 0.2.0 is a clean slate. Post-0.2.0, all public-API changes (rule IDs, JSON keys, `terrain.yaml` schema, CLI flags) follow the one-cycle deprecation contract per §10 and §6.

**LB-9: Runtime budget with per-phase decomposition.** End-to-end: `terrain test` cold-start ≤ 5s; per-PR analysis ≤ 60s on a 50k-file repo; resident memory ≤ 4 GB on standard CI runners. **Per-phase budgets** (measured per release on dogfood repos): graph construction ≤ 20s; rule evaluation ≤ 30s; output rendering ≤ 5s; remaining 5s buffer for I/O and orchestration. Regression past any phase budget on the dogfood repos is a release blocker. Adopters with very-large monorepos (>100k files) are supported on a best-effort basis; scale work is future.

**LB-10: Failure mode is fail-closed by default, with config opt-out.** When Terrain itself crashes, panics, or errors mid-run, the gate fails closed (status check is red, blocking merge) and emits a clear annotation explaining the bug and pointing to the issue tracker. Adopters who can't tolerate Terrain bugs blocking merges set `on_terrain_error: pass` in `terrain.yaml`; the default is `on_terrain_error: block`.

**LB-11: Bidirectional cause attribution validated.** The §1 mission commits to both FE→AI and ML→FE/data cause attribution. The harness runs scripted PRs from both directions on `terrain-testing-fullstack-rag`: (a) FE form-field change → `regression/eval-regression` finding fires with cause-path including the FE-changed file; (b) prompt-template edit → impact selection includes the FE component(s) whose input feeds it. Both directions must pass before 0.2.0 ships.

**LB-12: Senior-decision-maker evaluation bar.** N=5 senior engineers / engineering managers / PMs (unfamiliar with Terrain, recruited from the same panel pool as LB-2) read `docs/OVERVIEW.md` + 3 sample readiness cards + `docs/LIMITATIONS.md` and correctly answer four questions: (1) what category of tool is this? (2) what trust profile does it commit to? (3) what would adopting it require of my team? (4) what could break under the stability contract? Bar: ≥4 of 5 panelists answer all four correctly. Validates §2's senior-decision-maker persona being served by load-bearing artifacts, not just descriptive copy.

---

## 13. Validation harness

The harness is what keeps the LB bars honest across releases. It lives in `harness/` (or a sibling `terrain-readiness` repo) and runs on every release.

### Components

| Piece | Function |
|---|---|
| **Test corpus** | Dogfood repos pinned to known SHAs, plus scripted PRs that introduce specific violations (one per rule) |
| **Hand-labeled green / failure corpus** | ≥100 PRs per dogfood repo (mix of intended-green and seeded-failure), hand-labeled "should produce no findings" or "should produce findings X, Y, Z." Lives in `harness/corpora/<repo>/labels.yaml`. **Realistic labeling effort:** ~30 min per PR × 100 PRs × 5 repos ≈ 250 person-hours. **Labeling is done by the maintainer directly** rather than a multi-labeler consensus protocol — the project's solo-execution reality (§6) makes single-labeler the canonical mode at 0.2.0. The `terrain-corpus vote` / `adjudicate` subcommands (future work per §13) are designed for future multi-labeler scaling, not 0.2.0. Effort budget is real but bounded; spaced across the release cycle as Tier 4 work, not a single push. LB-3, LB-5 (FP), and LB-6 (recall) all measure against the labeled corpus. Corpus size of 100 (not 50) is the minimum needed for LB-5's Wilson 95% lower-bound at 5% to be statistically meaningful per §12. |
| **Runner** | Invokes Terrain on each PR (CI mode + CLI mode), captures all outputs |
| **Validators (automated)** | Schema check, silence check, reproduction-parity diff, Wilson-95% FP-rate computation (per-rule per-repo with min-sample-size floor), recall computation on seeded-failure corpus, renderer-conformance check against `dorny/test-reporter` (GH), `mikepenz/action-junit-report` (GH), and GitLab's native JUnit consumer, per-phase runtime measurement (graph / rules / render / total), failure-mode probe, bidirectional-cause-attribution scripted-PR runs (FE→AI and prompt-edit→FE), senior-decision-maker comprehension panel |
| **Triage panel (manual)** | External panel of engineers measuring LB-2a/b/c and LB-12. Given solo-execution funding reality (§6), the 0.2.0 model is a **volunteer panel of ~10 engineers** recruited from OSS / developer-tools communities, with optional small honoraria (e.g., $25-50 gift cards per session) rather than full paid-contractor rates. Capacity at this scale: 30 stable rules × N=3 panelists × ~2 repos sampled per rule ≈ ~180 panelist-rule readings per validation cycle. *N=3 instead of N=5* is the trade-off honest about solo-execution constraints; the readiness card reports N alongside the measured value so adopters know the statistical confidence. **If volunteer recruitment fails or yields too few participants, the cadence drops** (per `docs/PRODUCT.md` §6 — never silently skip; document in release notes). The plan's earlier 25-panelist paid-contractor sizing was unrealistic for solo-funded OSS; this adjustment is correctness-over-schedule applied to the validation infrastructure. |
| **Report generator** | Produces the per-rule readiness card, committed alongside the release tag |

### Triage panel protocol (LB-2)

- **Panelist eligibility:** engineers familiar with at least one similar tool (eslint, clippy, mypy, cargo) but *not* with Terrain itself.
- **Onboarding:** 5-minute Terrain orientation before timing begins.
- **Per-rule sample:** N=5 panelists minimum. Median time across panel is the reported number.
- **Timing window:** from "see CI failure" to "name the cause and a candidate fix." Stop the clock when the panelist articulates both.
- **Ground truth:** each rule has a pre-recorded "actual cause was X" label; the panelist's named cause is graded against it.
- **Compensation:** volunteer panel with optional small honoraria (gift cards) per session, **not full paid-contractor rates**, given solo-execution funding constraints (§6). Recruitment, honoraria amounts, and protocol documented in `harness/triage-panel.md`. **Funding source:** maintainer's personal budget at 0.2.0 release — kept small and bounded by the volunteer model. Post-1.0, project will consider sponsorship / foundation grant / community-supported models documented in `docs/CONTRIBUTING.md`. If volunteer recruitment fails, panel cadence drops with explicit acknowledgment in release notes — **never silently skipped**.
- **Cadence:** once per release tag at minimum. Spot checks per patch release only when a stable rule's detection mechanism changes. Once-per-release at higher sample-size yields better statistical power and is operationally sustainable for an OSS project. Pre-1.0 the project publishes actual panel-run dates and outcomes in each release's readiness cards.

### JUnit conformance target (LB-7)

JUnit XML has no canonical schema. Terrain pins to a specific renderer set:
- **GitHub:** `dorny/test-reporter` and `mikepenz/action-junit-report` (the two most commonly used GH JUnit consumers)
- **GitLab:** native JUnit test report (built into MR pipeline UI)

Integration tests in CI validate that Terrain's output renders correctly in both. Other JUnit renderers may work but aren't part of the conformance contract.

### Per-rule readiness card

The artifact that makes the bar real. Format (one per stable rule per release):

```
RULE: regression/eval-regression
TIER: stable
SEVERITY: error (default)

Diagnostic schema valid:        ✓
Reproduction parity (CI ↔ CLI): ✓
JUnit conformance:              ✓
FP rate (synthetic-green):      2.1%  (target: ≤5%)  ✓
Triage benchmark median:        43s   (target: ≤60s) ✓
Stable since:                   v0.2.0
Last validated:                 v0.2.0 (2026-XX-XX)
```

Preview rules get partial cards (no FP target, marked "preview — pending validation").

### Cadence

The harness runs on every release. Readiness cards are committed alongside the release tag. Adopters can read them before enabling preview rules or deciding to require the status check.

### Calibration corpus infrastructure (`terrain-corpus`, `terrain-precision`)

The harness depends on a labeled real-repo precision corpus that drives per-detector precision floors. This infrastructure ships with the project as two binaries — but the **scope at 0.2.0 is minimal**:

- **`cmd/internal/terrain-corpus/`** — at 0.2.0 ships only two of its nine subcommands: `extract` (pulls candidate findings from snapshots into the labeling pipeline) and `gate` (enforces precision floors against the labeled corpus). The remaining 7 subcommands (`sample`, `vote`, `adjudicate`, `aggregate`, `diff`, `regen`, `mine`) are active-learning and corpus-scaling infrastructure that 0.2.0's hand-labeled-by-maintainer corpus doesn't need; they are future work when corpus scaling becomes necessary.
- **`cmd/internal/terrain-precision/`** — at 0.2.0 ships `score` and `compare` only (compare detector configurations against labeled corpus, produce Wilson-95% precision metrics). `setup-fixtures` and `run` are full benchmark-harness orchestration; future work alongside the active-learning subcommands.

Both tools are committed (no longer untracked), documented in `docs/HARNESS.md`, and treated as load-bearing infrastructure for LB-5 (false-positive budget measurement) — at the reduced 0.2.0 subcommand scope. They are *maintainer tools*, not adopter tools. The full 13-subcommand surface lands incrementally as corpus-scale needs justify.

### Public release artifacts

To set the quality bar for the category, the following artifacts ship with every release:

- **Per-rule readiness cards** published alongside the release tag (not just internally maintained). Adopters and external observers can inspect exactly how each rule performs on the dogfood corpora: measured FP rate, measured triage time, last-validation date.
- **Open-sourced labeled calibration corpus** published as a reference artifact for the industry. "Here is a labeled corpus of AI/ML PRs with TP/FP labels; we use it to calibrate Terrain; you can use it to calibrate your own tools." Creates a public benchmark where none exists today. License: CC-BY-SA 4.0 or equivalent permissive.
- **Reproducible performance benchmarks** with public methodology — specific numbers per repo profile (cold-start time, per-PR analysis time, memory) with the harness scripts so adopters can run them themselves.

---

## 14. Dogfood repos

Multiple private repos in a `terrain-testing` GitHub org, each representing a different adopter profile.

| Repo | Profile | What it validates |
|---|---|---|
| `terrain-testing-fullstack-rag` | TS frontend + Python backend + RAG + evals | Unified-graph product story; **bidirectional cause attribution** — FE→AI direction (FE form-field change produces `regression/eval-regression` with cause-path including the FE file) AND reverse direction (prompt edit produces impact-selection including FE callers). Both validated per LB-11. |
| `terrain-testing-go-monolith` | Go service; unit + integration tests; no AI | Non-AI test detection rigor; "no AI surfaces → skip AI gate" |
| `terrain-testing-ai-only` | Promptfoo-driven Python AI library, no FE/BE | AI-side detection in isolation; eval-framework adapter shape |
| `terrain-testing-polyglot-monorepo` | TS + Python + Go in one repo | Cross-language detection, monorepo-shape selection |
| `terrain-testing-ml-pipeline` | Python + dbt + Airflow + classical ML training + MLflow registry | Pipeline awareness, registry integration, classical-ML cause attribution |

**All five are required for 0.2.0.** Cutting any of them cuts the category claim — the ML-pipeline profile in particular validates the classical-ML side of the unified-graph story, without which "AI+ML testing" is misleading.

Each repo:
- Is a real adopter-shape repo, not a synthetic meta-repo
- Has Terrain installed and configured normally
- Has scripted PRs producing specific findings for harness validation
- Has a hand-labeled corpus (per §13) used as the FP measurement substrate

### Sourcing

Mix of forked-OSS and bespoke, depending on availability:

| Repo | Source plan |
|---|---|
| `terrain-testing-go-monolith` | Fork of a permissive-licensed (MIT/Apache-2.0/BSD; **not GPL/AGPL**) Go service with non-trivial test coverage. License-audited per candidate. |
| `terrain-testing-polyglot-monorepo` | Fork of a permissive-licensed polyglot monorepo with TS + Python + Go presence. License-audited. |
| `terrain-testing-fullstack-rag` | **Bespoke** — no clean OSS option exists for "TS frontend + Python backend + RAG + evals + non-trivial test coverage." Built from scratch by the project; intended to be open-sourced once stable as a reference adopter setup. |
| `terrain-testing-ai-only` | **Bespoke** — built from scratch: Python AI library wrapping a real LLM-driven task (summarization or classification), with promptfoo evals exercising it. Decision committed (rather than depending on which OSS example apps prove suitable); the bespoke approach gives full control over the test substrate for harness validation. |
| `terrain-testing-ml-pipeline` | **Bespoke** — built from scratch: Python training pipeline using sklearn or XGBoost, dbt models providing training data, Airflow orchestrating the pipeline, MLflow registry tracking models, pytest tests for the training code, and Pandera or Great Expectations validating data quality. Intended to be open-sourced as the reference classical-ML adopter setup. |

License audit per fork happens before the repo is committed to. Forks are kept reasonably fresh (rebased against upstream periodically) so they stay representative.

### Third-party adopter repo for performance benchmarks

Performance benchmarks measured only on bespoke vendor repos lack adopter-shape credibility. **At least one third-party real-adopter repo (under NDA, not open-sourced) is part of the 0.2.0 performance-benchmark substrate.** Recruited from a willing early adopter; used for LB-9 runtime measurement and reproducibility but not for FP-rate measurement (their PRs aren't labeled). The published performance numbers in readiness cards include "measured on dogfood repo X" *and* "measured on third-party adopter repo Y (anonymized)" rows side by side. Senior decision-makers reading the benchmarks see vendor-substrate numbers and one independent reference.

### Reference public artifacts

Beyond the validation function, the dogfood repos serve a second purpose: once stable, the bespoke ones (`fullstack-rag`, `ml-pipeline`, possibly `ai-only`) are **open-sourced as reference integrated repos**. Adopters can clone them as starting points for their own Terrain-adopting projects. This is part of the category-defining commitment — making the "ideal adopter setup" visible and copyable.

---

## 15. Rule-doc page template

Each rule has a canonical page at `docs/rules/<rule-id>.md`. The runtime canonical URL in shipped findings is the GitHub permalink form (`https://github.com/pmclSF/terrain/blob/main/docs/rules/<category>/<rule-name>.md`), pinned to the release tag. The template serves both the developer hitting the finding and the senior decision-maker evaluating Terrain.

### Sections (every rule page, in order)

1. **Summary** — one-line description.
2. **Severity & status** — error/warning/off default; stable or preview; stable-since version.
3. **What this catches** — 3–5 concrete examples in plain language.
4. **Why this matters** — the problem class. References to incident patterns or industry consensus where applicable. Reads like a postmortem retrospective, not marketing.
5. **Detection mechanism** — AST scan / regex / threshold / data flow. Transparent so engineering teams trust it.
6. **Worked example** — failing diagnostic body, before/after fix.
7. **Configuration** — `terrain.yaml` snippets for severity, threshold tuning, ignore lists.
8. **False-positive characterization** — known patterns where the rule trips falsely, and how to handle them.
9. **Reproducibility** — `terrain test` command to reproduce locally.
10. **Stability commitment** — semver promise: when this rule's ID, severity, or behavior could change, and the deprecation path.
11. **Related rules** — cross-references in the catalog.

### Length

- Stable: 800–1500 words
- Preview: 400–600 words (some sections marked "preview — pending validation")

### Tone

Matter-of-fact engineering documentation. The first three rule pages written (`regression/eval-regression`, `regression/test-failed`, `coverage/no-tests`) serve as canonical references; the rest are templated against them.

### Templating discipline

- **Shared template:** `docs/rules/_template.md` — a real Markdown file in the repo with the 11 sections in canonical order, with placeholder content and inline guidance.
- **Per-rule narrative slots:** each rule page is a copy of the template with the slots filled in. Stable rules fill all 11 sections fully (~800–1500 words). Preview rules fill the **short-form template** — sections 1 Summary, 2 Severity, 3 What this catches, 5 Detection mechanism, 6 Worked example, 9 Reproducibility (~250 words total). The omitted sections (4 Why this matters depth, 7 Configuration depth, 8 FP characterization, 10 Stability commitment, 11 Related rules) carry a "preview — completed at graduation to stable" stub. This caps preview-doc effort substantially relative to the earlier 400–600-word preview bar.
- **Integration docs templating:** the same discipline applies to `docs/integrations/<tool>.md` files. `docs/integrations/_template.md` defines canonical sections (overview, install, terrain.yaml wiring, what Terrain consumes, what Terrain adds, troubleshooting, version compatibility). The 7 integration docs at 0.2.0 are template-fills, not independent essays — same maintenance model as rule pages.
- **Initial write:** bounded by the rule count (75 pages total — 30 stable + 45 preview). The template is designed first and validated against the three reference pages (`regression/eval-regression`, `regression/test-failed`, `coverage/no-tests`); the remaining 72 are produced against the locked template. **Preview rule pages are shorter** — per the templating discipline, preview rules fill only sections 1–3 + 5–6 + 9 (Summary, Severity, What this catches, Detection mechanism, Worked example, Reproducibility) with explicit "preview — pending validation" notes on the omitted sections (Why this matters, FP characterization, Configuration depth, Stability commitment, Related rules). Stable rules fill all 11 sections. This caps preview-doc effort at ~250 words per preview rule instead of 400–600.
- **Maintenance:** changes to a rule's behavior require a doc update in the same PR. Changes to the template propagate to all rule pages via a scripted update where possible (consistent sections), or via documented manual review where prose is involved.

---

## 16. 0.2.0 scope

**Scope of 0.2.0:** the unified-graph product end-to-end — a CI gate developers can't ignore, cross-stack cause attribution that actually works, a catalog of 75 rules (30 stable, 45 preview) serving both LLM and classical-ML adopters out-of-box, and public quality artifacts that make the trust profile auditable.

0.2.0 is the foundational release. Subsequent releases are additive — full IDE integration, marketplace listings, observability and production-aware integrations, additional language coverage, rule graduations. The headline use case (FE-developer-detects-AI-regression) works end-to-end at 0.2.0, the rule catalog is at the documented stable count, the public artifacts are published, or 0.2.0 doesn't ship. The principle is *correctness over schedule* (§6); the work takes the time it takes.

### Must-ship dependency-order spine

Several must-ship items transitively depend on others. The release work follows this critical path; items at each tier unblock the next:

1. **Tier 0 — Foundation** (must land before anything in Tier 1 is measurable):
   - `ImpactGraph` consulted by selection (CH5 fix) — unblocks `regression/test-failed`
   - Java import-graph linkage (CH3) — unblocks Java rules
   - Proper TOML/PEP-508 parsers (CH7/CH12) — unblocks `reproducibility/version-floating`
   - AST-based AI surface detection — unblocks honest cause-path narratives
   - Vocabulary rename + rule-ID namespace migration
2. **Tier 1 — Edges and adapters** (require Tier 0):
   - Cross-language API edges (OpenAPI / tRPC / gRPC / GraphQL / HTTP-route inference)
   - DB / schema awareness; DAG / pipeline awareness
   - Registry integration (MLflow, W&B, SageMaker, Vertex)
   - Eval-framework adapters (promptfoo, deepeval, ragas, Great Expectations; gauntlet as JSON-format-only per §17 note)
   - ML library detection (sklearn, PyTorch, etc.); model artifact awareness
3. **Tier 2 — Rules** (require Tiers 0 + 1):
   - 30 stable rules with detection mechanisms wired through Tier 0/1 infrastructure
   - 45 preview rules implemented + short-form docs
4. **Tier 3 — Surfaces** (require Tier 2):
   - JUnit XML emission + four-renderer diagnostic format + SARIF for `security/*`
   - MCP server with documented tool inventory
   - VS Code extension alpha (or deferred — see VS Code decision below)
5. **Tier 4 — Validation** (require Tiers 0–3):
   - Five dogfood repos built, including bidirectional-validation scenarios
   - Hand-labeled corpus (≥100 PRs × 5 repos)
   - Triage panel recruited + first run
   - Public readiness cards generated; LB-1 through LB-12 measured

Items within a tier can parallelize; items across tiers cannot. Effort estimates remain out of this document per *correctness over schedule*.

### Must-ship for 0.2.0

**The gate primitive:**

| Item | Load-bearing requirement |
|---|---|
| Single CI workflow (`terrain.yml`); single status check (`terrain/tests`) | Branch protection can require |
| JUnit XML emission | Validates and renders in supported renderer set (LB-7); net-new code |
| **SARIF emission for `security/*` rules** | Emit SARIF 2.1.0 alongside JUnit for `security/*` findings so adopters can route them to GitHub's Security tab via `github/codeql-action/upload-sarif`. JUnit remains primary (per "would pytest do this?"); SARIF is additive, opt-in via `--sarif <path>`. |
| Diagnostic format spec implemented | All four renderers produce the proposed output for stable rules |
| Annotation prioritization | Cap-handling per §8 |
| Silence-on-green | LB-3 met across labeled corpus on all dogfood repos |
| Unified JSON output (`version: 1`) | Single `impacted` array; first stable JSON schema; no aliases (0.2.0 is clean slate). **Includes warning-tier findings** (per R3-G1 fix — JUnit reserves `<failure>` for errors; warnings live in `findings.json` so the artifact-as-handoff contract carries both severities). |
| **`findings.json` schema as load-bearing artifact** | Documented schema with `version: 1` stability commitment; carries warning-tier findings (which JUnit does not), full `Finding` shape per §8, consumed by `terrain explain --from-run` and the MCP server. Net-new must-ship item. |
| `terrain.yaml` v1 schema | Parser, validator, severity / ignore / threshold support; tunable blocks per stable rule |
| Failure-mode behavior | LB-10 met; `on_terrain_error: block` default |
| **CLI surface implementation** | `terrain test`, `terrain explain`, `terrain describe`, `terrain accept-snapshot`, `terrain init`, `terrain --print-network` — all net-new subcommands in `cmd/terrain/main.go` (none exist today per audit). Stable from 0.2.0 release. |
| **Posture-band reconciliation** | Two parallel vocabularies (`measurement.PostureBand` vs `impact.postureBand`) unified into one taxonomy. Audit 4 surfaced the collision; resolve before 0.2.0 ships. |
| **`ImpactGraph` consulted by selection** | Fix `findImpactedTests` / `findCoveringTests` to use `ImpactGraph` rather than direct-scan `LinkedCodeUnits`. The "unified graph" claim is structurally true only after this fix (audit CH5). |
| **`decision.action` / `PolicyDecision` unification** | The two parallel "decision" concepts collapse into one with consistent vocabulary (audit 4). |

**Rule catalog:**

| Item | Load-bearing requirement |
|---|---|
| 30 stable rules | Detection + doc page + worked example + dogfood test case + LB-2 + LB-5 measured at target per rule. Includes 8 rules mapped from existing `internal/quality/` detectors, plus 3 classical-ML rules graduated from preview. Three originally-proposed classical-ML / hygiene rules stay preview because their mechanisms depend on infrastructure not landing in 0.2.0 (see §9). **The "30" is a ceiling, not a floor** — per the per-rule-per-repo LB-5 contract, rules that fail their FP bar on any dogfood repo at release-validation time are demoted to preview honestly. The released stable count is whatever clears the bar; 30 represents our intent and the work scope, not a release-blocker promise. |
| 45 preview rules implemented and short-form-documented | Full detection + short-form doc page per rule (~250 words; per §15 templating discipline); LB-2/LB-5/LB-6 not yet measured at target |
| Vocabulary rename | "scenario" → "eval"; "AI surface" → "surface"; direct rename (no alias period — clean slate). Affects: Go type names (`models.Scenario` → `models.Eval`), JSON output keys (`ai.selectedScenarios` → `ai.selectedEvals` and similar), CLI flag/subcommand names where applicable, YAML config keys, workflow files, test fixtures, and ~70 rule doc pages. Real refactor across the codebase; tracked as a single coordinated work item rather than incremental renames. |
| Rule-ID namespace migration | All existing `TER-<DOMAIN>-<NNN>` IDs renamed to `terrain/<category>/<rule>` (audit 4 / CH2) |

**Unified-graph foundations:**

| Item | Load-bearing requirement |
|---|---|
| Cross-language API edges | TS/JS ↔ Python/Go/Java edges via OpenAPI, tRPC, gRPC `.proto`, GraphQL schemas, and HTTP-route inference. The headline FE→AI use case requires this. **Field-level narrowing primitive:** route-level reachability alone over-selects impacted tests. Edges carry a `fields_read` set extracted from schema where available (OpenAPI body schema, tRPC type, gRPC message fields, GraphQL selection sets); rules like `regression/test-failed` filter impact selection to changes touching at least one field actually consumed downstream. For HTTP routes without schema, fall back to route-level (and document the limitation in the rule's FP-characterization page). |
| DB / schema awareness | SQL migration parsing (Postgres/MySQL DDL); ORM model parsing for sqlc, gorm, prisma, sqlalchemy. Column changes propagate to surfaces that read the column. Audit 3b: zero today. |
| DAG / pipeline awareness | dbt manifest.json parsing; Airflow / Prefect DAG parsing. Upstream data changes propagate to downstream models. Audit 3b: zero today. |
| AST-based AI surface detection | Replaces regex-based detection across supported languages. Existing regex detection becomes a fallback. Required for honest cause attribution. |
| Registry integration | MLflow + W&B + SageMaker + Vertex registry awareness sufficient for `lifecycle/*` and `hygiene/model-fixture-unpinned`. Audit 3a: MLflow and W&B detected today; SageMaker and Vertex registry are net-new. |
| Per-framework eval adapters | **Four named adapters** — promptfoo, deepeval, ragas, Great Expectations. Each with documented end-to-end workflow, not just parse support. **Gauntlet supported as a JSON-format-compatible ingestion path**, not as a named adapter — the project commits to gauntlet's JSON output shape as a stable input contract (so adopters using gauntlet can wire it in), but does not commit "gauntlet adapter" as a first-class artifact. Rationale: gauntlet (Mosaic Eval Gauntlet, in `mosaicml/llm-foundry`, Apache-2.0) is tightly coupled to MosaicML's training stack (Composer / FSDP / llm-foundry runtime), not a standalone eval harness adopters drop in like promptfoo. Terrain commits to its JSON output contract rather than its runtime requirements. Adopters effectively get gauntlet support; the commitment surface is the JSON contract, not the toolchain. |
| **Java import-graph linkage** | Add `.java` to import-extraction switch (audit CH3). Without this, Java tests have no impact linkage at all. |
| **Java test file-pattern expansion** | `*IT.java`, `*Spec.java`, `*Tests.java` (audit CH9). Common conventions silently dropped today. |
| **LLM SDK source-scan for Go and Java** | Expand allowlist beyond `.ts/.js/.py/.tsx/.jsx/.mjs` (audit CH8). |
| **Proper TOML / lockfile parsing** | Replace `strings.Contains` with PEP-508 / TOML parsers (audit CH7/CH12). Required for `reproducibility/version-floating` honesty. |
| **Broader pydantic detection** | Extend from LLM-output-only to general data-validation use (audit CH13). Required for `hygiene/missing-validator`. |
| **HuggingFace disambiguation** | Separate `transformers` non-LLM usage (BERT classifier, etc.) from LLM usage (audit CH14). |
| **ML library detection** (audit 3a — net-new) | Detection for sklearn, xgboost, lightgbm, statsmodels (classical ML); PyTorch, TensorFlow, JAX (deep learning). All absent today. |
| **Model artifact awareness** | Recognize `.pt`/`.pth`/`.h5`/`.joblib`/`.onnx`/`.safetensors` as model artifacts (currently absent; `.pkl`/`.pickle` are mis-classified as datasets per audit 3a). |

**LLM and agent surfaces:**

| Item | Load-bearing requirement |
|---|---|
| LLM tier infrastructure | All three tiers per §11. Provider abstraction pinned to chat completions + tools. |
| Ollama / BYOK / OpenAI-compatible HTTP | All providers in §11 matrix validated |
| `terrain describe` (one-time descriptions) | First-run UX with security-friendly defaults; produces committed `surfaces:` block. Net-new — descriptions don't currently live anywhere on `CodeSurface` nodes (audit 4). |
| `terrain explain` (CLI enrichment) | Reads CI artifact; LLM-mediated narrative; never runs in CI |
| **MCP server as reference implementation** | Exposes diagnostic tools to MCP-aware agents (Claude Code, Cursor). **Pinned spec version: MCP 2025-11-25** (latest stable per spec.modelcontextprotocol.io at the time of plan-writing; final pinned version reconfirmed at release tag). **Minimum tool inventory (load-bearing):** `list_findings`, `get_finding(id)`, `get_cause_path(finding_id)`, `read_surface(name)`, `read_eval(name)`, `read_baseline(eval, ref)`, `suggest_action(finding_id)`, `reproduction_command(finding_id)`. Ships with copyable Claude Code / Cursor configs and 3 example agent-workflow transcripts in `docs/integrations/mcp.md`. The project commits to a one-cycle deprecation when adopting a new spec version that breaks compatibility (per §18 versioning). |
| **VS Code extension at 0.2.0 — Marketplace publication, not sideload alpha** | A sideload-only VSIX won't reach the FE-developer persona the mission targets. 0.2.0 ships a Marketplace-published alpha with the minimum capability set documented below, so the mission's "unifies testing *and development*" line has a real, discoverable referent at launch. **Minimum capability set:** (a) reads Terrain JUnit/JSON artifacts from disk or last-CI-run; (b) renders findings in the IDE Problems pane with severity-correct icons; (c) click-to-navigate to the cause-path primary location; (d) hover shows `Finding.short_message` + suggested action. *Not* in 0.2.0 alpha: inline squigglies, on-save analysis, refactoring actions, full LSP-server mode. Marketplace listing is clearly tagged "0.2.0 alpha — limited scope." |
| **Headline-case end-to-end harness test** | Net-new must-ship item per v3 R1-I3. A scripted PR on `terrain-testing-fullstack-rag` modifies a FE form field (e.g., removes the input-length cap on `CommentInput.tsx`); the harness runs Terrain and verifies (a) `regression/eval-regression` fires on the downstream `summarize_refusal` eval, (b) the finding's `cause_path` contains the FE-changed file as the originating node, (c) the diagnostic renders correctly across all four diagnostic surfaces. This is the single scriptable proof that the §1 headline case works in shipped product. Run on every release. Bidirectional sibling test (prompt edit → FE callers impacted) validates LB-11. |

**Internal renames and cleanup** (audit-driven, free at clean-slate moment):

| Item | Load-bearing requirement |
|---|---|
| `internal/migration/` → `internal/framework_migration/` | Avoid name collision with future SQL/DB migration work (audit 4) |
| `internal/analysis/schema_parser.go` → `internal/analysis/ai_schema_parser.go` | Avoid collision with future SQL-schema parsing (audit 4) |
| Dead code: `ExternalServiceNodeCount` | Either wire to populated source or remove (audit 3b — currently set up but never populated) |
| Dead code: `terrain-precision` `terrain-regex` no-op config | Implement or remove (audit 4) |

**Catalog of public artifacts** (category-defining additions):

| Item | Load-bearing requirement |
|---|---|
| Public per-rule readiness cards | Published alongside the release tag; adopters and observers can inspect each rule's measured FP rate, triage time, validation date |
| Open-sourced labeled calibration corpus | The `terrain-corpus` output published as a reference public benchmark; license CC-BY-SA 4.0 or equivalent |
| Reproducible public performance benchmarks | Specific numbers per dogfood-repo profile with public methodology; adopters can run the harness themselves |

**Tooling integration docs** (named workflows, not just detection):

| Item | Load-bearing requirement |
|---|---|
| `docs/integrations/promptfoo.md` | End-to-end workflow: install promptfoo, configure for Terrain, what Terrain consumes, what Terrain adds. *Eval framework.* |
| `docs/integrations/deepeval.md` | Same shape. *Eval framework.* |
| `docs/integrations/ragas.md` | Same shape. *Eval framework.* |
| `docs/integrations/gauntlet.md` | Same shape, with note on gauntlet's runtime coupling: Mosaic Eval Gauntlet ships inside `mosaicml/llm-foundry` (Apache-2.0) and depends on Composer / FSDP / MosaicML's training stack. Terrain's commitment is to gauntlet's JSON output format, not to running gauntlet itself; adopters who use the full MosaicML toolchain wire gauntlet's outputs into Terrain via the JSON-format-compatible adapter. *Eval framework.* |
| `docs/integrations/great-expectations.md` | Same shape. *Data-validation framework.* |
| `docs/integrations/mlflow.md` | How Terrain reads from the MLflow registry, what it surfaces. *Model registry.* |
| `docs/integrations/wandb.md` | Same shape. *Model registry / experiment tracking.* |

**Documentation:**

| Item | Load-bearing requirement |
|---|---|
| `docs/PRODUCT.md` | This document |
| `docs/OVERVIEW.md` | 1–2 page senior-decision-maker doc; designed to convert in under 5 minutes |
| `docs/rules/` (75 pages) | All rules — 30 stable, 45 preview — per the §15 template; stable pages full treatment, preview pages short-form per §15 templating discipline |
| `docs/rules/_template.md` | Canonical rule-page template |
| `docs/LIMITATIONS.md` | Honest public list of what Terrain doesn't do in 0.2.0. Builds trust. |
| `docs/WALKTHROUGH.md` | Worked end-to-end demo: `git clone` of a dogfood repo, run PR analysis, see diagnostics, run `terrain explain`, see agent integration. Concrete, reproducible, screenshotted/recorded. |
| `docs/HARNESS.md` | Validation harness documentation: how the dogfood corpora are built, how readiness cards are produced, how the triage panel works |
| `docs/integrations/_template.md` | Canonical integration-doc template (parallel to the rule template); ensures the 7 integration docs share consistent structure |
| `docs/integrations/mcp.md` | MCP server tool inventory, version-tracking policy, example agent configs (per T2.10) |
| `SECURITY-DATA-HANDLING.md` | Covers LLM-provider AND CI-provider data flow (source code excerpts in JUnit/annotations leaving the dev's machine to GitHub/GitLab/CircleCI). Includes `redact_source: true` config option. |
| `harness/triage-panel.md` | Triage panel recruitment, compensation, and protocol |
| `CHANGELOG.md` | Explicit statement that 0.2.0 is the first stable release (no pre-0.2.0 compatibility) |
| `README.md` rewrite | Anchored on the unified-graph product story; passes the **"FE dev who's never heard of this" test:** an engineer unfamiliar with Terrain reads the first 200 words and can correctly answer three questions: (1) "what category of tool is this?" (2) "is this relevant to my team's work?" (3) "what would adopting this require of me?". Validated with N=5 unfamiliar engineers (same panel pool as LB-2) before 0.2.0 ships. |

**Operational infrastructure:**

| Item | Load-bearing requirement |
|---|---|
| Validation harness | All components per §13; readiness cards generated for all stable rules (30 at ship intent; actual count = those clearing LB-5); published with release |
| All 5 dogfood repos | Per §14 — none deferred; hand-labeled corpora present; harness runs against all five |
| `terrain-corpus` and `terrain-precision` committed and documented | Per §13 — load-bearing harness substrate, not optional |
| Reference integrated repos (public) | Three reference public repos open-sourced from the bespoke dogfood set (`terrain-testing-fullstack-rag`, `terrain-testing-ml-pipeline`, and the bespoke version of `terrain-testing-ai-only`). Each demonstrates Terrain configured end-to-end for its profile (eval framework wired, MLflow integrated, branch protection set, CI runs visible). Adopters fork the one matching their stack as their starting point. |

### Explicit non-goals for 0.2.0

| Deferred to | Item |
|---|---|
| Future | Capabilities not in scope for 0.2.0 are described in §17. |

### Canonical CLI surface for 0.2.0

The complete adopter-facing CLI for 0.2.0. Each command is stable from the release tag forward per §6:

**Primary analysis and explanation:**

| Command | Purpose |
|---|---|
| `terrain test [path]` | Run analysis, emit findings to terminal (cargo-style) |
| `terrain test --json` | JSON output (stable schema, `version: 1`) |
| `terrain test --junit <path>` | Emit JUnit XML to specified path |
| `terrain test --selector <pattern>` | Run subset of rules or paths |
| `terrain test --severity <level>` | Filter by minimum severity (error / warning / notice) |
| `terrain test -v` / `--verbose` | Verbose terminal output |
| `terrain test --base <ref>` | Base ref for diff (default: detected from CI / `main`) |
| `terrain test --head <ref>` | Head ref (default: `HEAD`) |
| `terrain explain <rule>` | Narrative explanation of a finding (LLM-enriched if configured) |
| `terrain explain --from-run <id>` | Read from CI artifact, no local reproduction needed |
| `terrain explain --from-json <path>` | Read from local JSON artifact |
| `terrain explain --eval <name>` | Explain a specific eval failure |

**Setup and configuration:**

| Command | Purpose |
|---|---|
| `terrain init` | Initialize `terrain.yaml` in current repo with sensible defaults |
| `terrain describe` | Generate surface descriptions (LLM-mediated) |
| `terrain describe --setup` | First-time setup flow with security-friendly defaults |
| `terrain describe --provider <name>` | Override provider for this run |
| `terrain accept-snapshot [snapshot-id]` | Accept a baseline change (`regression/snapshot-mismatch`) |
| `terrain accept-snapshot --all` | Bulk accept all pending snapshot changes |
| `terrain accept-snapshot --review` | Interactive review mode |

**Meta and audit:**

| Command | Purpose |
|---|---|
| `terrain --version` | Print version |
| `terrain --help` | Help text |
| `terrain --print-network` | Audit: list every external call Terrain would make under current config (zero by default with templates-tier only) |

**Conversion subsystem CLI** (ships in 0.2.0 as-is, separate product narrative):

| Command | Purpose |
|---|---|
| `terrain convert [path]` | Convert tests between frameworks (e.g., Jest → Vitest) |
| `terrain migrate [path]` | Multi-step migration with progress tracking |
| `terrain detect [path]` | Detect framework in test files |
| `terrain convert-config` | Migrate framework configuration files |
| `terrain list` / `shorthands` / `estimate` / `status` / `checklist` / `doctor` / `reset` | Conversion subsystem helpers; see conversion docs |

**Maintainer-only binaries** (separate `cmd/` binaries, not part of adopter CLI):

| Binary | Purpose |
|---|---|
| `cmd/internal/terrain-corpus` | Calibration corpus management (nine subcommands per §13) |
| `cmd/terrain-precision` | Detector precision benchmarking (four subcommands per §13) |

### Tier 5 — Corpus-driven quality (must-ship for 0.2.0 release)

The 0.2.0 calibration corpus (326 repos, 104,420 labeled PRs) was originally Tier 4 work. The data it surfaced names six concrete quality gaps that block 0.2.0 from being a primary CI gate. These are blocking, not deferred — Tier 5 is now also must-ship. Each item cites the tier-4 evidence motivating it.

The 0.2.0 release-condition target: **measured corpus recall on regression-introducing PRs ≥ 50%** (today: 31.7%). Items 1 and 3 below are the single biggest unlocks.

**Tier 5.1 — Dependency-drift detector** *(closes 35.2% of the recall gap)*

  - Evidence: `tier-4/recall-gap.md` — 27.1% of unflagged regression PRs are bot-authored; another 8.1% are deps-bump-only. `tier-4/author-lift.md` — renovate PRs have **38.8% regression rate** vs 8% baseline; Terrain fires on 2.7% today.
  - Detector: flags bot-authored PRs that touch dependency-manifest files (`package.json` / `requirements.txt` / `Cargo.toml` / `go.mod` / `pyproject.toml` / etc.) with a major-version bump or with version-range widening.
  - Rule ID: `terrain/deps/drift-risk` (stable on 0.2.0 ship).
  - Acceptance: corpus PR-lift ≥ 1.5x with CI lower bound > 1.0; combined-recall increase ≥ 15pp on the regression-PR set.

**Tier 5.2 — Config-schema-change detector** *(closes another 5.7% of the gap)*

  - Evidence: `tier-4/recall-gap.md` — 376 unflagged regression PRs are config-only edits (YAML / TOML / JSON / INI / TF).
  - Detector: flags PRs that change top-level schema in a tracked-config file — key renames, key removals, non-additive type changes. Distinguishes "added a new key" from "renamed an existing key."
  - Rule ID: `terrain/config/schema-drift` (stable on 0.2.0 ship).
  - Acceptance: corpus PR-lift ≥ 1.5x; eliminates the config-only gap class from the recall-gap clusters.

**Tier 5.3 — Re-base severity on empirical lift, not detector source declarations** *(fixes "critical = ignore" trust problem)*

  - Evidence: `tier-4/severity-calibration.md` — across 9,281 reg + 111,603 safe PRs, `critical` findings have **0.00× lift** (fire only on safe PRs). 92.5% of findings emit as `medium`. The ladder has no predictive power.
  - Change: severity is derived per-detector from `internal/explain/data/detector-evidence.json` (corpus lift + survival rate), not from hard-coded `Confidence:` in detector source. Rules with corpus lift CI lower-bound > 1.5 default to `high`; > 1.2 default to `medium`; ≤ 1.0 default to `low`. Detectors with no measured lift (<100 firings in corpus) explicitly emit `severity: preview`.
  - Acceptance: `--fail-on=high` selects findings with median corpus lift > 1.5x; one-cycle deprecation of old detector-source-driven path.

**Tier 5.4 — Trim `terrain explain` output to 4 always-on + 3 verbose-only sections** *(scanability)*

  - Evidence: `cmd/terrain/cmd_explain.go` accumulated 9 sections during corpus work. Default render is ~15-20 lines per finding.
  - Change: always-on = id+location, why-it-matters, what-to-do, next-steps. Behind `--verbose` = detector-evidence, lineage, OSS examples. Cross-detector evidence stays always-on when pair stacking gain ≥ 1.5× (the 17.14× signal we measured).
  - Acceptance: a typical finding renders ≤ 8 lines in default mode.

**Note on Tier 5.5 / 5.6 (status discipline + language scoping):** earlier drafts of this section included two items demoting under-validated AI detectors and adding language-scope caveats. Those were reverted: we are not pre-emptively diminishing Terrain's claims before doing the work to validate / improve the detectors. Items 5.5/5.6 remain candidates if expanded-corpus validation proves the detectors aren't earning their stable status; until then, focus stays on building.

### Release condition

0.2.0 ships when every requirement in §12 is satisfied for every must-ship item including Tier 5 — not before. There is no fixed schedule. *Correctness over schedule* (§6) is the controlling principle: a release blocker is a release blocker. The correct response to schedule pressure is scope reduction (deferring rules from stable to preview, or from preview to a later release), never lowering a quality bar on what does ship.

The Tier 5 release-condition addition: **measured regression-PR recall ≥ 50% on the 326-repo corpus** (running tally in `tier-4/recall-final.md`).

Effort estimates are deliberately not included in this document. They invite the wrong conversation. The work takes the time it takes.

---

## 17. Beyond 0.2.0

0.2.0 ships the unified-graph product end-to-end. Subsequent releases are *additive* — they extend coverage, add surfaces, graduate preview rules, and broaden ecosystem reach. None of them are foundational architecture work 0.2.0 deferred.

Each future release is defined by *scope*, not by date. Capability lands when its scope items meet the load-bearing quality bars. Rules graduate from preview to stable when their measurements clear the target on the dogfood repos.

### Integration philosophy

Beyond CI, tests run and deploy in many places — dbt projects, ML platforms, data-quality systems, deployment orchestrators, observability tools. Terrain's value scales with how many of these systems it can read from and integrate with. The integration philosophy: **Terrain reads test-and-deploy artifacts from where they actually live, surfaces them in the unified graph, and gates on them through the same CI primitive.** Each integration follows a consistent shape: ingest the tool's artifact, map to the graph, surface in rules.

### Out-of-scope integrations (explicit non-goals)
- **CodeRabbit, Greptile, Sourcegraph** — adjacent / overlapping code-review tools. Terrain replaces parts of these for the AI/ML CI gate use case rather than integrating.
- **Snyk, Trivy, Dependabot, FOSSA** — vulnerability and license scanning. Terrain integrates with adopters' existing tools (does not duplicate). See §11.
- **Postman, Hoppscotch, Insomnia** — API testing platforms. Not in scope unless meaningful adopter demand.

---

## 18. Project operations

Operational dimensions adopters and contributors need to know.

### License

Terrain ships under **Apache 2.0**. Permissive license with explicit patent grant. Standard for OSS developer tools and unambiguous for commercial adopters. License text in `LICENSE` at repo root.

The published labeled calibration corpus (§13) ships under a separate **CC-BY 4.0** license (corrected from earlier draft — CC-BY-SA was copyleft and would restrict commercial use; CC-BY is permissive with attribution requirement). Corpus contributors retain credit; downstream users can redistribute and modify freely with attribution.

### Governance

- **Maintainer:** project owner (current: pmclSF; expand as additional maintainers join).
- **RFC process:** significant changes go through GitHub RFCs in a `rfcs/` directory before merge. "Significant" includes: new stable rules, breaking changes to any public API, new public artifacts, new integrations beyond bug fixes. Trivial changes (typo fixes, doc updates, bug fixes to existing rules) do not require RFCs.
- **Rule lifecycle:** new rules proposed via GitHub Issue + RFC. New rules ship as preview by default. Graduation to stable requires LB-2 / LB-5 measurement at target on the dogfood repos via the harness.
- **Disagreements:** resolved by the maintainer; final decisions documented as `accepted: false` RFCs with rationale, so the project has a public record of "we considered this and declined."

### Versioning

Semantic versioning, with explicit pre-1.0 stability commitments:

- `0.x → 0.(x+1)` is a minor version bump but treated as a **major-version-equivalent** for stability: breaking changes follow the one-cycle deprecation process per §6 even pre-1.0.
- `0.x.y → 0.x.(y+1)` is a patch: no breaking changes; bug fixes and additions only.
- Internal APIs (under `internal/`) are not part of the stability contract; only public APIs (rule IDs, JSON output schema, `terrain.yaml` schema, CLI flags, artifact format) carry the commitment.
- 1.0 is reserved for when the unified-graph architecture is stable across all planned cross-stack edge types and the test-conversion subsystem is also production-stable.

### Telemetry

**None by default.** Terrain does not phone home, collect anonymous usage statistics, or report crashes unless the adopter explicitly opts in via a documented configuration flag. This is a deliberate trust signal:

- Zero outbound network calls in default configuration. Verifiable via `terrain --print-network`.
- No crash-reporting service integration.
- Local logs only; nothing leaves the dev's machine.

Future opt-in telemetry (if added) will be documented in `SECURITY-DATA-HANDLING.md` and require explicit `terrain.yaml` enablement, never default-on.

### Issue triage and response

Documented in `docs/CONTRIBUTING.md`. Targets for 0.2.0:

| Issue type | Acknowledgment | Target fix |
|---|---|---|
| Security vulnerability | 24 hours | 7 days (critical) / 30 days (high) |
| Bug in stable rule (false positive at scale, false negative) | 48 hours | best-effort, prioritized by impact |
| Bug in preview rule | 1 week | best-effort, may be addressed via rule revision |
| Feature request | 1 week | discussed via RFC process |
| Documentation issue | 48 hours | best-effort within minor release cycle |

These are targets, not guarantees. They scale with maintainer capacity. Pre-1.0, the project commits to publishing actual response-time data per release in the release notes (`CHANGELOG.md`), with a single annual aggregate in a `docs/RESPONSE-REPORT.md` summarizing the year's triage outcomes. Adopters can hold the project accountable to its stated targets via these public artifacts.

### Documentation organization

Doc files referenced throughout this plan, organized by audience:

**Adopter / decision-maker documentation:**
- `README.md` — top-level entry; 200-word pitch for the FE-dev test
- `docs/OVERVIEW.md` — 1–2 page senior decision-maker doc
- `docs/PRODUCT.md` — this document; canonical product reference
- `docs/WALKTHROUGH.md` — worked end-to-end demo, screenshotted/recorded
- `docs/LIMITATIONS.md` — honest list of what 0.2.0 doesn't do
- `docs/CHANGELOG.md` — versioned change history
- `MIGRATING.md` — *not in 0.2.0* (0.2.0 is first stable release)

**Per-rule documentation:**
- `docs/rules/_template.md` — canonical rule-page template
- `docs/rules/<rule-id>.md` × 73 — one page per rule

**Per-integration documentation:**
- `docs/integrations/promptfoo.md`, `deepeval.md`, `ragas.md`, `gauntlet.md`, `great-expectations.md`, `mlflow.md`, `wandb.md` (7 files at 0.2.0; more as integrations land in later releases)

**Security and trust:**
- `SECURITY.md` — coordinated disclosure policy
- `SECURITY-DATA-HANDLING.md` — data-flow doc for security review (LLM provider + CI provider)

**Internal / maintainer:**
- `docs/HARNESS.md` — validation harness internals
- `docs/CONTRIBUTING.md` — how to contribute, RFC process, issue triage
- `harness/triage-panel.md` — triage panel recruitment and protocol
- `rfcs/` — RFC documents directory

**Architecture (separate from product story):**
- `DESIGN.md` — technical architecture document (existing; not rewritten in 0.2.0 work)

---

## 19. Open questions

Most questions in earlier drafts have been answered by the codebase audits (cross-stack, ML-lifecycle, test detection, infrastructure inventory, existing-code description) and operational research. The plan also reflects two important constraints made explicit in the principles (§6):

- **Solo execution.** Terrain is built by a single engineer with AI-assistant collaboration. Tier 0→4 dependency-spine is sequential; effort is bounded by single-engineer attention. Earlier plan drafts assumed team capacity; that's been corrected.
- **OSS-only dependencies.** No proprietary code libraries. Commercial APIs (OpenAI, Anthropic, etc.) acceptable as opt-in BYOK paths because adopters bring their own keys; the project itself doesn't depend on vendor relationships.

Remaining operational items below are bounded operational outreach / research; nothing architectural is open.

1. **License-compatibility audit per dogfood-repo fork candidate.** Specific OSS repos to fork for `terrain-testing-go-monolith` and `terrain-testing-polyglot-monorepo` need to be shortlisted, and each candidate's license verified compatible (MIT / Apache-2.0 / BSD; not GPL/AGPL). Operational, not architectural.
2. **Triage panel recruitment.** Sourcing ~25 engineers willing to participate in paid rotating 60-minute triage sessions (cadence: once per release tag, spot checks per patch only when rule mechanisms change). Pilot recruitment can begin in parallel with implementation work; panel must be operational by 0.2.0 release for LB-2a/b/c measurement. Funding source pinned per §13: maintainer-funded for 0.2.0, longer-term funding model documented in `docs/CONTRIBUTING.md` pre-1.0.
3. **MCP spec version pin and stability monitoring.** ~~MCP is evolving. Track current spec version and breaking-change risk between now and 0.2.0; pick a specific version to pin against.~~ **Resolved:** 2025-11-25 confirmed as latest stable; no newer dated spec released; 2026 MCP roadmap is working-group-driven (additive enterprise/audit-trail/Streamable-HTTP work, not breaking). Pin documented in `docs/integrations/mcp.md`. Subsequent spec changes follow the one-cycle deprecation contract per §18. Pin reconfirmed at each release tag.
4. **Provider matrix validation.** ~~The §11 matrix lists Ollama / vLLM / etc. tool-calling support based on documentation; smoke-test against current versions before 0.2.0 ship to confirm.~~ **Resolved via §11 matrix update:** Anthropic native API recommended for production (OpenAI-compat is parity-only); Ollama tool calling is first-class (not partial — quality varies by model); vLLM `strict` field accepted but unenforced (use JSON-schema-guided decoding). LM Studio remains partial-quality for tools. Validated against current provider docs.
5. **Detection-library embedding (Presidio path).** ~~gitleaks is a Go library and embeds cleanly. Microsoft Presidio is Python — confirm subprocess-invocation approach is acceptable, or pick an alternative PII library with a Go binding.~~ **Resolved:** gitleaks confirmed embeddable via `github.com/zricethezav/gitleaks/v8/detect` (v8.30+ recommended; API stable since 2022; pin minor version in `go.mod` and revisit per release). Presidio path resolved separately per §9 `security/pii-in-eval` mechanism: Go-native default detector ships at 0.2.0; Presidio remains opt-in via subprocess for adopters needing higher recall.
6. **Open-corpus data-provenance review.** The corpus license (CC-BY 4.0) is decided per §18. Open item: legal review of the corpus contents themselves — code snippets extracted from real repos via `terrain-corpus extract` may carry their original repos' licenses, which interact with publication. Resolution required before publication, not before code-ships.
7. **Adopter base size for pre-0.2.0** (residual from earlier draft). ~~With 0.2.0 declared the first stable release, this is less load-bearing — the doc no longer commits to MIGRATING.md from pre-0.2.0. Still worth measuring for CHANGELOG communication tone.~~ **Resolved:** public signals (GitHub: 4 stars / 1 fork / 1 contributor; no Homebrew formula; no related npm package; no community mentions; ~38 lifetime binary downloads across pre-0.2.0 tags) confirm adopter base is effectively zero (0–5 people, plausibly all known to the maintainer). **Tone for CHANGELOG: clean-slate framing.** No migration guide warranted; a short note that 0.2.0 is the first release intended for real use is sufficient.

Questions resolved by the audits and incorporated into the plan:
- Per-rule readiness — addressed in §9 mechanism notes
- Non-AI test detection rigor — addressed in §16 must-ship (Java import-graph, file-pattern expansion)
- ML-lifecycle coverage — addressed in §16 must-ship (ML library detection, registry integration)
- Cross-stack edge gap — addressed in §16 must-ship (cross-language API edges, DB awareness, DAG awareness)
- Existing-code disposition — addressed in §16 must-ship (internal renames, posture-band reconciliation, decision unification)
- `terrain-corpus` / `terrain-precision` disposition — addressed in §13 (committed, documented as harness substrate)

---

## 20. Glossary

| Term | Definition |
|---|---|
| **Adopter** | An organization or developer using Terrain in their repository |
| **Cause path** | Ordered chain of graph nodes from a finding's primary location back to the change in the PR that caused it |
| **Eval** | Any oracle that produces a verdict on a surface's behavior — LLM scenario, test-set evaluation, data validation, drift check, fairness check |
| **Finding** | One result from one rule, with location, evidence, suggestions; renders to four CI/CLI/agent surfaces |
| **LB-N** | Load-bearing quality requirement N (see §12) |
| **Metric** | A score produced by an eval — rubric score, accuracy, F1, drift KL, latency, etc. |
| **Preview rule** | A documented, opt-in rule with lower validation than stable; ships in the catalog to signal scope |
| **Readiness card** | Per-rule per-release validation artifact produced by the harness |
| **Rule** | A configurable detection capability with a stable ID, severity default, and doc page |
| **Stable rule** | A rule validated to the full LB bar; default-on; production-ready |
| **Surface** | A point in the codebase where an AI/ML system is exposed |
| **Tier** | Stable or preview |
| **Triage benchmark** | The time required for an unfamiliar engineer to identify cause and fix from a finding's CI surface alone |
| **Unified graph** | The dependency graph spanning code, tests, surfaces, evals, data, and (in later releases) cross-language and infra-layer edges |
