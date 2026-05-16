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

## Capabilities deferred to a later release

Capability we plan to add but is not in 0.2.0. Linked to the roadmap section in `docs/PRODUCT.md` §17.

### Deferred to 0.3.0

- **Full IDE integration.** 0.2.0 ships a VS Code Marketplace extension at *alpha* — reads artifacts and renders findings in the Problems pane, but no inline squigglies, no on-save analysis, no refactoring actions, no LSP-server mode. Full LSP + JetBrains + Neovim + Helix in 0.3.0.
- **Additional language support.** Ruby, Rust, Kotlin. 0.2.0 supports Go, JS/TS, Python, Java.

### Deferred to 0.4.0+

- **Polyrepo support.** 0.2.0 analyzes a single repo. Adopters with FE/BE in separate repos run Terrain on each independently. Cross-repo edges (shared graph spanning multiple repos) are 0.4.0+ work.
- **GitHub Marketplace Action.** Publication of `pmclSF/terrain-action@v1`. 0.2.0 ships Terrain as a binary that adopters invoke directly from CI; the named Action wrapper lands later.
- **Claude Skill / OpenAI Apps SDK listings.** Marketplace listings that wrap the MCP server. 0.2.0 ships the MCP server itself; the vendor-marketplace packaging lands later.
- **Additional eval framework adapters.** Evidently, deepchecks, Fairlearn, NannyML. 0.2.0 ships four named adapters (promptfoo, deepeval, ragas, Great Expectations) plus gauntlet via JSON-format-compatible ingestion.
- **Deeper dbt integration.** 0.2.0 parses dbt's `manifest.json` (structure). Ingesting `dbt test` runtime results as Terrain findings, plus full lineage edges to dependent code, lands in 0.4.0.
- **GraphQL ecosystem runtime integration.** 0.2.0 parses GraphQL schemas for cross-language edges. Ingesting Apollo / Hasura / GraphQL-test-suite *results* lands in 0.4.0.
- **Test-execution platform ingestion.** Cypress Cloud, Playwright Cloud, BrowserStack, Sauce Labs run results. 0.4.0+.

### Deferred to 0.5.0+

- **Observability tool integration.** Honeycomb, Datadog, New Relic, Sentry, Grafana, Prometheus. Production-aware rules (`performance/production-latency-regression`, `regression/production-error-spike`, `coverage/production-call-without-monitor`) require this and are reserved as rule IDs but not implemented at 0.2.0.
- **Deployment-orchestrator integration.** Argo CD, Spinnaker, Flagger pre-deploy hooks. Terrain as a deploy-gate (not just merge-gate). 0.5.0.
- **Data-observability integration.** Monte Carlo, Bigeye, Soda, Anomalo, Datafold.
- **Coverage data ingestion.** Codecov, Coveralls.
- **Deep ML platform integration.** SageMaker Pipelines / Model Monitor / Clarify / Endpoints; Vertex AI Pipelines / Model Monitoring / Explainable AI / Endpoints; Databricks ML / Unity Catalog; Azure ML Pipelines & Model Registry. 0.2.0 ships only the *registry-awareness* layer (MLflow, W&B detected; SageMaker / Vertex registry awareness added).
- **LLM-observability integration.** Langfuse, Helicone, Arize Phoenix, OpenLLMetry, OpenInference.
- **Cost tracking integration.** Vantage, CloudZero, OpenCost.
- **Issue-tracker integration.** Linear, Jira (for "create issue from Terrain finding" flow).
- **Intra-procedural data-flow tracing.** Required for `security/insecure-deserialization` to do *content-aware* detection (resolves path to user-controlled input). At 0.2.0 the rule is purely structural (any unguarded pickle.load is flagged); precision refinement lands when data-flow tracing lands.

### Deferred to 0.6.0+

- **Additional language coverage.** Swift, Scala, C# per adopter demand.
- **Additional registry / MLOps integrations.** Comet, Neptune, ClearML, DagsHub, Modal.
- **Feature-store integration.** Feast, Tecton, Hopsworks, Featureform.
- **Vector-database integration.** Pinecone, Weaviate, Chroma, Qdrant, Milvus, pgvector.
- **ML governance integration.** Fiddler, Arthur, WhyLabs, Truera.
- **Notebook environment integration.** Jupyter, Hex, Deepnote, nbval, testbook.
- **Additional pipeline orchestrators.** Kubeflow, Metaflow, ZenML, Flyte.
- **Visual / UI testing integration.** Percy, Chromatic, Applitools.
- **Contract testing integration.** Pact, Spectral.
- **Notifications.** Slack, MS Teams, PagerDuty — configurable per-rule, on-error-only. Deliberate non-default: silence-on-green principle holds; notifications are opt-in.
- **Additional CI platform support.** Bitbucket Pipelines, Jenkins, Azure Pipelines, Buildkite, TeamCity. 0.2.0 supports GitHub Actions and GitLab CI.

## Rules in *preview* at 0.2.0 (not default-on)

45 rules ship as preview — fully implemented and documented in short-form, but default-off because LB-2a/b/c and LB-5 (FP rate at Wilson 95% lower bound) have not been measured at the target bar on the dogfood repos at release time. Adopters can opt in via `terrain.yaml`; their feedback feeds graduation to stable.

See `docs/PRODUCT.md` §9 preview-rule list for the full set. The largest preview categories:

- **Fairness** (4) — `group-disparity`, `missing-group-eval`, `disparate-impact`, `group-coverage-low`. Stay preview until Fairlearn / Aequitas detection lands.
- **Drift / data validation** (8) — `drift-detected`, `calibration-degraded`, `schema-mismatch`, `null-rate-high`, `distribution-shift`, `duplicate-rows`, `imbalanced-classes`, `feature-leakage`, `group-leakage`. Stay preview until Evidently / Alibi Detect / NannyML detection lands and Pandera / Great Expectations adapters mature past parse-only.
- **Lifecycle** (6) — `model-not-registered`, `missing-monitoring`, `orphaned-artifact`, `no-rollback-plan`, `missing-shadow-mode`, `no-deprecation-path`. Stay preview until cross-system registry integrations land.
- **Performance** (5) — `latency-regression`, `throughput-regression`, `memory-regression`, `cost-regression`, `training-time-regression`. Stay preview until observability ingestion lands.

## Specific known limitations at 0.2.0

These are not "deferred to later" — they are explicit limitations of how 0.2.0 capabilities are *bounded*.

- **Cross-language API edges are inferred at route granularity** for HTTP routes without schema. OpenAPI / tRPC / gRPC / GraphQL get field-level narrowing (the `fields_read` set); raw HTTP routes don't. Adopters with substantial untyped HTTP traffic between languages will see `regression/test-failed` over-select tests on FE changes touching those routes.
- **`data/leakage-suspected` covers row-overlap and temporal leakage only** at 0.2.0. Feature leakage (column derived from label) and group leakage (same entity in both splits) are preview rules `data/feature-leakage` and `data/group-leakage`.
- **`security/insecure-deserialization` is structural at 0.2.0** — flags any unguarded `pickle.load` / `joblib.load` / `torch.load` / `yaml.load` regardless of whether the path resolves to user-controlled input. Higher FP rate on legitimate trusted-load patterns; adopters can suppress via path-level ignore. Content-aware refinement lands with intra-procedural data-flow tracing (0.5.0+).
- **MCP spec version is pinned to 2025-11-25.** Newer MCP spec versions adopt via the one-cycle deprecation contract; adopters using newer MCP clients should consult `docs/integrations/mcp.md` for compatibility notes.
- **VS Code extension is alpha.** Marketplace-published but with limited capability — reads artifacts, renders Problems-pane findings, click-to-navigate. No inline squigglies, no real-time analysis. Full extension lands in 0.3.0.
- **The validation harness measures against the dogfood corpora.** LB-5 FP rate, LB-2 triage time, LB-6 recall are all measured against the project's curated test substrate plus one third-party adopter repo (under NDA). They are *predictive of* real adopter behavior but are not adopter-specific. Adopters can run the harness against their own repos to get repo-specific measurements.
- **Telemetry is none-by-default; there is no anonymous usage data.** Adopters who want to share usage stats with the project to inform development priorities can opt in (not yet implemented; future feature behind explicit config). The project cannot tell, by default, how many adopters use Terrain or which rules they've configured.

## Things the project will *not* do under stability commitments

- **Will not silently change rule behavior.** Detection-mechanism changes that alter findings on previously-unchanged input go through one-cycle deprecation.
- **Will not silently rename rule IDs.** Renames are aliased for one minor version with stderr warnings; documented in `CHANGELOG.md`.
- **Will not break the `terrain.yaml` schema without a deprecation cycle.** v1 schema is stable from 0.2.0 release.
- **Will not change the JUnit / SARIF / `findings.json` artifact contract without a deprecation cycle.**
- **Will not introduce mandatory cloud dependencies.** Default operation is fully offline.
- **Will not publish adopter data.** The labeled calibration corpus contains data from real OSS repos under permissive licenses, not adopter repos; adopter usage is not aggregated or shared.

---

*If something feels missing from this list — i.e., you expected Terrain to do X and it doesn't — file a GitHub issue. Adding it here is part of the *auditable quality* commitment.*
