# Terrain — Product reference

> *Public product reference for Terrain. Technical architecture detail lives in [`../DESIGN.md`](../DESIGN.md). This document covers the mission, principles, scope, and stability commitments.*

## 1. Mission

**Terrain is a pre-flight check for AI/ML systems and the tests around them.** A static analyzer that runs locally and in CI, with **no LLM API key required**, ever. It treats unit tests, integration tests, end-to-end tests, ML model evaluations, data validation, and LLM eval scenarios as nodes in **a single dependency graph**, and surfaces failures in that graph as a **real CI gate** — not as a comment, dashboard, or email.

The headline use case: a frontend developer changes a form field and learns, *in their PR*, that their change will degrade a downstream AI model's behavior — before merge. The reverse is equally true: an ML engineer who edits a prompt or retrains a model learns which upstream code paths and downstream features are affected.

Terrain achieves this without ever calling an LLM at scan time: the analysis is structural (import-graph, AST, schema parsing), not generative.

## 2. Audience

Terrain serves five personas working in the same repository:

- **Frontend developer.** Doesn't know the AI codebase; needs to know if their change breaks an AI contract.
- **Backend / platform engineer.** Owns the data layer; needs to know which models are affected by a schema or API change.
- **ML engineer (classical ML).** Changes feature engineering or trains a new model; needs regression detection on test-set metrics, drift, and fairness.
- **ML engineer (LLMs).** Edits prompts, RAG pipelines, or model fixtures; needs scenario-based regression detection.
- **Senior decision-maker (CTO, Principal Engineer, PM).** Evaluating Terrain for adoption; needs to understand scope, trust, false-positive cost, and stability guarantees.

The first four interact with Terrain through failing tests and PR diagnostics. The fifth reads the docs and the readiness cards.

### Languages analyzed

Terrain's detector engines analyze: **Python, TypeScript, JavaScript, Go, Java**. Code in other languages (Rust, Kotlin, Swift, C / C++, Ruby, etc.) is not currently analyzed.

## 3. Three co-equal product goals

Terrain commits to three goals as load-bearing, all measured at every release:

1. **Unified graph.** Code, tests, AI surfaces, evals, and data live in one dependency graph. Cross-language edges (TS/JS ↔ Python/Go/Java) via OpenAPI / tRPC / gRPC / GraphQL / HTTP-route inference, plus database schema and pipeline awareness.
2. **Real CI gate.** Output is failing test cases in the platform's Tests tab — not narrative review comments. Blocks merge on the same primitive as any other test runner.
3. **Auditable quality.** Per-rule readiness cards measure false-positive rate and triage time on dogfood repos and are published with each release. The labeled calibration corpus is open-source.

## 4. Non-goals

These will not be added to Terrain. Adopters needing them should look elsewhere.

- **Hosted SaaS.** Terrain is OSS that runs in the adopter's CI and on developer machines. No hosted-service version. No paid tier.
- **Telemetry by default.** Terrain does not phone home. Future opt-in telemetry, if added, will require explicit `terrain.yaml` enablement and will be documented in `SECURITY-DATA-HANDLING.md`.
- **Vulnerability scanning** beyond the in-scope `security/*` rules. Terrain does not duplicate Snyk / Trivy / Dependabot / FOSSA; adopters integrate those independently.
- **License compliance scanning.** Same — integrate with FOSSA or equivalent.
- **General code-review commentary.** Terrain's output is failing test cases with structured diagnostics, not narrative reviews.
- **Generic-purpose AI tooling.** Terrain integrates with eval frameworks but is not itself one.

## 5. Vocabulary

| Term | Meaning |
|---|---|
| **Signal** | Atomic observation emitted by a detector (e.g., "test file imports framework X"). |
| **Finding** | A signal raised to merge-gate visibility, with severity + cause path + reproduction command. |
| **Rule** | Configurable detection capability with stable ID (`terrain/<category>/<rule>`), severity default, and doc page. |
| **Tier** | Stable or preview. Stable rules ship default-on at measured quality; preview rules ship default-off as scope-under-evaluation. |
| **Surface** | A code or AI target: prompt, agent, tool, context, scenario, code unit, test file. |
| **Cause path** | Chain of graph nodes from a finding's primary location back to the change in the PR that caused it. |
| **Unified graph** | Dependency graph spanning code, tests, surfaces, evals, data, and cross-language edges. |
| **Posture** | Five-dimension band (health, coverage depth, coverage diversity, structural risk, operational risk) computed per repo. |

## 6. Principles

- **Calibration over intuition.** Every shipped rule's false-positive rate is measured against a labeled corpus before it is promoted from preview to stable.
- **Local-first, no required keys.** Every gate finding, every detector, every PR-comment surface must work without an LLM API key. Optional LLM-enhanced features layer on when a user brings their own key — never gating core value.
- **Stability is a public contract.** Rule IDs, JSON output schema, `terrain.yaml` schema, and CLI flags are stable from 0.2.0 forward. Changes follow a one-cycle deprecation with stderr warnings.
- **Detectors are redesigned, not retired.** Low-precision detectors enter redesign cycles. The observability tier is the safety net; the gate panel earns its place via measurement.
- **Failures are loud.** When Terrain itself crashes or errors mid-run, the gate fails closed (status check red) and emits a clear annotation. Adopters who can't tolerate this set `on_terrain_error: pass` in `terrain.yaml`.
- **Findings carry evidence.** Every finding includes a cause path, the signals that produced it, and a reproduction command. `terrain explain` surfaces this directly.

## 7. Architecture — three-surface model

Terrain emits the same diagnostic artifact through three surfaces:

- **CI surface.** GitHub Actions status check + JUnit XML + GitHub annotations + Step Summary. The merge gate.
- **CLI surface.** `terrain analyze`, `terrain test`, `terrain explain`, etc. Local reproduction parity with CI.
- **Agent surface.** MCP server exposing diagnostic tools to MCP-aware agents (Claude Code, Cursor, etc.) — read-only.

The artifact format (JUnit + `findings.json`) is the handoff contract. All three surfaces read it; no surface invents new diagnostic semantics.

## 8. Diagnostic format

Each finding includes:

- `rule_id` — stable identifier.
- `severity` — clause-backed severity assignment (see [severity-rubric.md](severity-rubric.md)).
- `primary_location` — file + line.
- `cause_path` — chain of graph nodes from the change to this finding.
- `short_message` — one-line description.
- `evidence` — references to the signals that produced this finding.
- `suggested_action` — fix-direction.
- `reproduction` — exact CLI command to reproduce this finding locally.
- `docs_url` — canonical rule page URL.

See the [finding schema](../schemas/finding.v1.json) for the formal contract.

## 9. Rule catalog

0.2.0 ships rules across ten categories: regression, coverage, hygiene, reproducibility, data, performance, fairness, security, lifecycle, and documentation. Stable rules ship default-on; preview rules ship default-off and graduate as their false-positive rate and triage time clear the load-bearing quality bars.

Per-rule documentation lives in [`docs/rules/<category>/<rule-name>.md`](rules/).

## 10. Configuration

`terrain.yaml` (versioned `v1`) declares the repo's surfaces, policy, and optional integrations:

```yaml
surfaces:
  summarizer:
    description: "Summarizes user-submitted comments; refuses harmful inputs."
    type: llm
  intent_classifier:
    description: "Routes incoming requests to downstream handlers."
    type: llm

policy:
  disallow_skipped_tests: error
  max_test_runtime_ms: 30000

integrations:
  promptfoo:
    config: promptfoo.config.yaml
```

The `surfaces:` `type:` enumeration is closed (`llm | classical_ml | deep_learning | rag_pipeline | feature_pipeline | prediction_service | data_validator`). Adding a new type requires a schema-version bump with a one-cycle deprecation.

## 11. Stability commitments

| Surface | Contract |
|---|---|
| Rule IDs | Stable across minor versions; renames follow a one-cycle deprecation with stderr warnings |
| JSON output schema | `version: 1`; one-cycle deprecation cycle on changes |
| `terrain.yaml` schema | Versioned `v1`; closed enumeration for surface types; one-cycle deprecation on changes |
| CLI flags | Stable from 0.2.0; same deprecation contract |
| Telemetry | None by default. No phone home, no usage stats. Verifiable via `terrain --print-network`. |
| Data flow | Templates tier: zero outbound network calls. Optional local-LLM tier (Ollama) is the documented default; other LLM tiers are explicit adopter choices. |

For the data-handling contract in full, see [`../SECURITY-DATA-HANDLING.md`](../SECURITY-DATA-HANDLING.md).

## 12. Quality

Each stable rule's measured false-positive rate (Wilson 95% lower bound on a labeled corpus) and median triage time are published as readiness cards alongside the release tag.

## 13. License

- **Terrain itself:** Apache 2.0. Permissive license with explicit patent grant. Unambiguous for commercial adopters.
- **Labeled calibration corpus:** CC-BY 4.0. Permissive with attribution requirement. Contributors retain credit; downstream users can redistribute and modify freely.

## 14. Versioning

Semantic versioning, with explicit pre-1.0 stability commitments:

- `0.x → 0.(x+1)` is a minor version bump but treated as a **major-version-equivalent** for stability: breaking changes follow the one-cycle deprecation process.
- `0.x.y → 0.x.(y+1)` is a patch: no breaking changes; bug fixes and additions only.

See [versioning.md](versioning.md) for the full contract.

## 15. Beyond 0.2.0

Subsequent releases are *additive* — they extend coverage, add surfaces, graduate preview rules, and broaden ecosystem reach. No foundational architecture work is deferred to subsequent releases.

Out-of-scope integrations (explicit non-goals across all future releases):

- **CodeRabbit, Greptile, Sourcegraph code-review** — adjacent / overlapping AI code-review tools. Terrain composes where possible; the AI/ML CI-gate use case overlaps with what AI-aware code-review bots do today.
- **Snyk, Trivy, Dependabot, FOSSA** — vulnerability and license scanning. Terrain integrates with adopters' existing tools.
- **Postman, Hoppscotch, Insomnia** — API testing platforms. Not in scope.

## 16. Related docs

- [`OVERVIEW.md`](OVERVIEW.md) — evaluator-focused summary.
- [`quickstart.md`](quickstart.md) — first report in five minutes.
- [`cli-spec.md`](cli-spec.md) — canonical CLI surface.
- [`LIMITATIONS.md`](LIMITATIONS.md) — what 0.2.0 does not do.
- [`severity-rubric.md`](severity-rubric.md) — severity labels and configuration.
- [`../DESIGN.md`](../DESIGN.md) — technical architecture.
- [`../CHANGELOG.md`](../CHANGELOG.md) — release history.
- [`rules/`](rules/) — per-rule documentation.
