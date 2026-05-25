# Limitations — what Terrain 0.2.0 does *not* do

> *Honest public list. Adopters should read this before deciding to adopt. Updated per release.*

This document is part of the §3 *auditable quality* product goal. The plan commits to publishing what the project doesn't do, not just what it does. When something on this list is addressed in a later release, it's removed from the list and added to the release's changelog.

## Categorically out-of-scope (not in the roadmap)

These will not be added to Terrain. Adopters needing them should look elsewhere.

- **Hosted SaaS.** Terrain is OSS that runs in the adopter's CI and on developer machines. There is no hosted-service version. No paid tier.
- **Telemetry by default.** Terrain does not phone home. Future opt-in telemetry, if added, will require explicit `terrain.yaml` enablement and be documented in `SECURITY-DATA-HANDLING.md`.
- **Vulnerability scanning** beyond the in-scope `security/*` rules. Terrain does not duplicate Snyk / Trivy / Dependabot / FOSSA; adopters integrate those independently.
- **License compliance scanning.** Same — integrate with FOSSA or equivalent.
- **General code-review commentary.** Terrain's output is failing test cases with structured diagnostics, not narrative reviews. Adopters wanting code-quality narrative use CodeRabbit / Greptile / similar.
- **Generic-purpose AI tooling.** Terrain integrates with eval frameworks but is not itself one. It does not author evals, manage prompts, or run experiments.

## Capabilities not in 0.2.0

These capabilities are not in scope for 0.2.0. They may land in future releases as priorities and adopter feedback warrant. The list below is illustrative of common asks, not a roadmap commitment.

**IDE and editor integrations**
- Full LSP-server mode, JetBrains, Neovim, Helix. 0.2.0 ships a VS Code Marketplace alpha that reads artifacts and renders Problems-pane findings.

**Languages beyond Go, JS/TS, Python, Java**
- Ruby, Rust, Kotlin, Swift, Scala, C# are not analyzed in 0.2.0.

**Multi-repo / polyrepo analysis**
- 0.2.0 analyzes a single repo. Adopters with FE/BE in separate repos run Terrain on each independently.

**Marketplace listings**
- GitHub Action / Claude Skill / OpenAI Apps SDK listings are not yet published. 0.2.0 ships Terrain as a binary invoked directly from CI; the MCP server is also shipped.

**Additional eval, data, and ML platform adapters**
- 0.2.0 includes promptfoo, deepeval, ragas, Great Expectations (plus gauntlet via JSON-compatible ingestion). Other eval frameworks (Evidently, deepchecks, Fairlearn, NannyML), data-observability tools, ML registries beyond MLflow/W&B, deeper dbt and GraphQL runtime integration are not in scope at 0.2.0.

**Observability and production-signal ingestion**
- Honeycomb, Datadog, New Relic, Sentry, Grafana, Prometheus, LLM-observability platforms, cost-tracking platforms. Production-aware rules that depend on these are not implemented at 0.2.0.

**Deeper data-flow analysis**
- Intra-procedural data-flow tracing is not in 0.2.0. `security/insecure-deserialization` is structural-only as a result (any unguarded `pickle.load` / `joblib.load` / `torch.load` / `yaml.load` is flagged).

**Other CI platforms beyond GitHub Actions and GitLab CI**
- Bitbucket Pipelines, Jenkins, Azure Pipelines, Buildkite, TeamCity are not packaged with first-class templates at 0.2.0.

## Rules in *preview* at 0.2.0 (not default-on)

A subset of rules ship as preview — fully implemented and documented in short-form, but default-off because triage time and false-positive rate have not been measured at the target bar at release time. Adopters can opt in via `terrain.yaml`; their feedback feeds graduation to stable.

The largest preview categories:

- **Fairness** (4) — `group-disparity`, `missing-group-eval`, `disparate-impact`, `group-coverage-low`. Stay preview until Fairlearn / Aequitas detection lands.
- **Drift / data validation** (8) — `drift-detected`, `calibration-degraded`, `schema-mismatch`, `null-rate-high`, `distribution-shift`, `duplicate-rows`, `imbalanced-classes`, `feature-leakage`, `group-leakage`. Stay preview until Evidently / Alibi Detect / NannyML detection lands and Pandera / Great Expectations adapters mature past parse-only.
- **Lifecycle** (6) — `model-not-registered`, `missing-monitoring`, `orphaned-artifact`, `no-rollback-plan`, `missing-shadow-mode`, `no-deprecation-path`. Stay preview until cross-system registry integrations land.
- **Performance** (5) — `latency-regression`, `throughput-regression`, `memory-regression`, `cost-regression`, `training-time-regression`. Stay preview until observability ingestion lands.

## Specific known limitations at 0.2.0

These are not "deferred to later" — they are explicit limitations of how 0.2.0 capabilities are *bounded*.

- **Cross-language API edges are inferred at route granularity** for HTTP routes without schema. OpenAPI / tRPC / gRPC / GraphQL get field-level narrowing (the `fields_read` set); raw HTTP routes don't. Adopters with substantial untyped HTTP traffic between languages will see `regression/test-failed` over-select tests on FE changes touching those routes.
- **`data/leakage-suspected` covers row-overlap and temporal leakage only** at 0.2.0. Feature leakage (column derived from label) and group leakage (same entity in both splits) are preview rules `data/feature-leakage` and `data/group-leakage`.
- **`security/insecure-deserialization` is structural at 0.2.0** — flags any unguarded `pickle.load` / `joblib.load` / `torch.load` / `yaml.load` regardless of whether the path resolves to user-controlled input. Higher FP rate on legitimate trusted-load patterns; adopters can suppress via path-level ignore. Content-aware refinement lands with future intra-procedural data-flow tracing.
- **MCP spec version is pinned to 2025-11-25.** Newer MCP spec versions adopt via the one-cycle deprecation contract; adopters using newer MCP clients should consult `docs/integrations/mcp.md` for compatibility notes.
- **VS Code extension is alpha.** Marketplace-published but with limited capability — reads artifacts, renders Problems-pane findings, click-to-navigate. No inline squigglies, no real-time analysis. Full extension capability is future work.
- **The validation harness measures against the project's curated test substrate.** False-positive rate, triage time, and recall are all measured there. They are *predictive of* real adopter behavior but are not adopter-specific. Adopters can run the harness against their own repos to get repo-specific measurements.
- **Telemetry is none-by-default; there is no anonymous usage data.** Adopters who want to share usage stats with the project to inform development priorities can opt in (not yet implemented; future feature behind explicit config). The project cannot tell, by default, how many adopters use Terrain or which rules they've configured.

## Things the project will *not* do under stability commitments

- **Will not silently change rule behavior.** Detection-mechanism changes that alter findings on previously-unchanged input go through one-cycle deprecation.
- **Will not silently rename rule IDs.** Renames are aliased for one minor version with stderr warnings; documented in `CHANGELOG.md`.
- **Will not break the `terrain.yaml` schema without a deprecation cycle.** v1 schema is stable from 0.2.0 release.
- **Will not change the JUnit / SARIF / `findings.json` artifact contract without a deprecation cycle.**
- **Will not introduce mandatory cloud dependencies.** Default operation is fully offline.
- **Will not publish adopter data.** Adopter usage is not aggregated or shared.

---

*If something feels missing from this list — i.e., you expected Terrain to do X and it doesn't — file a GitHub issue. Adding it here is part of the *auditable quality* commitment.*
