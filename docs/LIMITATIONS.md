# Limitations — what Terrain 0.3.0 does *not* do

> *Honest public list. Adopters should read this before deciding to adopt. Updated per release.*

This document is part of the *auditable quality* product goal: publish what the project doesn't do, not just what it does. When something on this list is addressed in a later release, it's removed from the list and added to the release's changelog.

## Categorically out-of-scope (not in the roadmap)

These will not be added to Terrain. Adopters needing them should look elsewhere.

- **Hosted SaaS.** Terrain is OSS that runs in the adopter's CI and on developer machines. There is no hosted-service version. No paid tier.
- **Remote telemetry.** Terrain does not phone home. Optional local telemetry can be enabled with `terrain config telemetry --on`; it writes to the adopter's own `~/.terrain/telemetry.jsonl` and is never sent by Terrain.
- **Vulnerability scanning** beyond the in-scope `security/*` rules. Terrain does not duplicate Snyk / Trivy / Dependabot / FOSSA; adopters integrate those independently.
- **License compliance scanning.** Same — integrate with FOSSA or equivalent.
- **General code-review commentary.** Terrain's output is failing test cases with structured diagnostics, not narrative reviews. Adopters wanting code-quality narrative use CodeRabbit / Greptile / similar.
- **Generic-purpose AI tooling.** Terrain integrates with eval frameworks but is not itself one. It does not author evals, manage prompts, or run experiments.

## Capabilities not in 0.3.0

These capabilities are not in scope for 0.3.0. They may land in future releases as priorities and adopter feedback warrant. The list below is illustrative of common asks, not a roadmap commitment.

**IDE and editor integrations**
- Full LSP-server mode, Problems-pane diagnostics, JetBrains, Neovim, Helix. 0.3.0 includes a VS Code extension alpha that renders sidebar tree views from CLI JSON and supports file reveal; Marketplace publication is future work.

**Languages beyond Go, JS/TS, Python, Java**
- Ruby, Rust, Kotlin, Swift, Scala, C# are not analyzed in 0.3.0.

**Multi-repo / polyrepo analysis**
- 0.3.0 ships portfolio aggregation via `terrain portfolio --from .terrain/repos.yaml`, including repo rollups, owner/tag propagation, snapshot-backed inputs, and framework-of-record drift findings. It does **not** compute cross-repo import/eval dependency edges; adopters with FE/BE in separate repos should still run Terrain on each repo independently for gate decisions.

**Marketplace listings**
- GitHub Marketplace Action / VS Code Marketplace extension / Claude Skill / OpenAI Apps SDK listings are not yet published. 0.3.0 ships Terrain as a binary invoked directly from CI, with workflow templates under `docs/examples/gate/`; the MCP server is also shipped.

**Additional eval, data, and ML platform adapters**
- 0.3.0 includes promptfoo, deepeval, ragas, Great Expectations (plus gauntlet via JSON-compatible ingestion). Other eval frameworks (Evidently, deepchecks, Fairlearn, NannyML), data-observability tools, ML registries beyond MLflow/W&B, deeper dbt and GraphQL runtime integration are not in scope at 0.3.0.

**Observability and production-signal ingestion**
- Honeycomb, Datadog, New Relic, Sentry, Grafana, Prometheus, LLM-observability platforms, cost-tracking platforms. Production-aware rules that depend on these are not implemented at 0.3.0.

**Deeper data-flow analysis**
- Intra-procedural data-flow tracing is not in 0.3.0. `security/insecure-deserialization` is structural-only as a result (any unguarded `pickle.load` / `joblib.load` / `torch.load` / `yaml.load` is flagged).

**CI templates beyond GitHub Actions**
- GitLab CI, Bitbucket Pipelines, Jenkins, Azure Pipelines, Buildkite, TeamCity are not packaged with first-class templates at 0.3.0. They can still run the Terrain CLI and consume JUnit/SARIF-style artifacts where the platform supports them.

## Rules in *preview* at 0.3.0 (not default-on)

A subset of rules ship as preview — fully implemented and documented in short-form, but default-off because triage time and false-positive rate have not been measured at the target bar at release time. Adopters can opt in via `terrain.yaml`; their feedback feeds graduation to stable.

The largest preview categories:

- **Fairness** (4) — `group-disparity`, `missing-group-eval`, `disparate-impact`, `group-coverage-low`. Stay preview until Fairlearn / Aequitas detection lands.
- **Drift / data validation** (8) — `drift-detected`, `calibration-degraded`, `schema-mismatch`, `null-rate-high`, `distribution-shift`, `duplicate-rows`, `imbalanced-classes`, `feature-leakage`, `group-leakage`. Stay preview until Evidently / Alibi Detect / NannyML detection lands and Pandera-style dataframe validation plus data-validation-specific detector wiring mature past parse-only.
- **Lifecycle** (6) — `model-not-registered`, `missing-monitoring`, `orphaned-artifact`, `no-rollback-plan`, `missing-shadow-mode`, `no-deprecation-path`. Stay preview until cross-system registry integrations land.
- **Performance** (5) — `latency-regression`, `throughput-regression`, `memory-regression`, `cost-regression`, `training-time-regression`. Stay preview until observability ingestion lands.

## Specific known limitations at 0.3.0

These are not "deferred to later" — they are explicit limitations of how 0.3.0 capabilities are *bounded*.

- **Cross-language API edges are inferred at route granularity** for HTTP routes without schema. OpenAPI / tRPC / gRPC / GraphQL get field-level narrowing (the `fields_read` set); raw HTTP routes don't. Adopters with substantial untyped HTTP traffic between languages will see `regression/test-failed` over-select tests on FE changes touching those routes.
- **`data/leakage-suspected` covers row-overlap and temporal leakage only** at 0.3.0. Feature leakage (column derived from label) and group leakage (same entity in both splits) are preview rules `data/feature-leakage` and `data/group-leakage`.
- **`security/insecure-deserialization` is structural at 0.3.0** — flags any unguarded `pickle.load` / `joblib.load` / `torch.load` / `yaml.load` regardless of whether the path resolves to user-controlled input. Higher FP rate on legitimate trusted-load patterns; adopters can suppress via path-level ignore. Content-aware refinement lands with future intra-procedural data-flow tracing.
- **MCP spec version is pinned to 2025-11-25.** Newer MCP spec versions adopt via the one-cycle deprecation contract; adopters using newer MCP clients should consult `docs/integrations/mcp.md` for compatibility notes.
- **VS Code extension is alpha.** Source and package metadata ship in the repo, but it is not Marketplace-published in 0.3.0. It renders sidebar tree views from CLI JSON and supports click-to-navigate. No Problems-pane diagnostics, inline squigglies, or real-time analysis. Full extension capability is future work.
- **Measured readiness cards are not published for every rule in 0.3.0.** Harness infrastructure exists, but false-positive rate, triage time, and recall should only be represented as published for rules with generated cards under `harness/readiness/v0.3.0/`. Until then, rule docs and feature status describe known false-positive patterns and measurement status rather than measured adopter-specific quality.
- **A few `terrain.yaml` fields parse but are inert today.** `redact_source: true`, `on_terrain_error: pass`, the `ai.framework / scenarios_dir / baselines_dir` block, the `ml.registry / artifacts_dir` block, and the `explain` block all parse cleanly but are not yet read by any emission or analyze path. Adopters who set them get the documented schema (no error on load) and no observable behavior change. The field names are reserved so future wiring lands without a config-schema migration.
- **Remote telemetry does not exist.** Optional local telemetry can be enabled explicitly, but it remains a local JSONL file. The project cannot tell how many adopters use Terrain or which rules they've configured unless an adopter deliberately shares that local file or a summary.

## Things the project will *not* do under stability commitments

- **Will not silently change rule behavior.** Detection-mechanism changes that alter findings on previously-unchanged input go through one-cycle deprecation.
- **Will not silently rename rule IDs.** Renames are aliased for one minor version with stderr warnings; documented in `CHANGELOG.md`.
- **Will not break the `terrain.yaml` schema without a deprecation cycle.** v1 schema is stable from 0.2.0 release.
- **Will not change the JUnit / SARIF / `findings.json` artifact contract without a deprecation cycle.**
- **Will not introduce mandatory cloud dependencies.** Default operation is fully offline.
- **Will not publish adopter data.** Adopter usage is not aggregated or shared.

---

*If something feels missing from this list — i.e., you expected Terrain to do X and it doesn't — file a GitHub issue. Adding it here is part of the *auditable quality* commitment.*
