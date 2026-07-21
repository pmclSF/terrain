# Terrain — Overview

> *For senior engineers, engineering managers, and product leaders evaluating Terrain for adoption. Read time: 5 minutes.*

## What Terrain is

**Terrain is a pre-flight check for AI/ML systems and the tests around them.** A static analyzer that inspects prompts, retrievers, classical-ML training pipelines, eval coverage, schema-code-test relationships, and schema↔prompt drift across files and languages — in your PR, before merge. Locally, deterministically, on every push. **No API key required.**

Terrain treats unit tests, integration tests, e2e tests, and AI/ML evals as nodes in a single dependency graph. A frontend developer learns in their PR if their change will degrade a downstream AI model's behavior. An ML engineer learns which upstream code paths their prompt edit affects. The analysis spans code, prompt, schema, and eval boundaries.

Terrain is the layer that connects existing evidence: it ingests Promptfoo, DeepEval, Ragas, Great Expectations, Gauntlet, coverage, runtime, schema, and source signals, then renders them as deterministic findings, CI test cases, explainable impact reports, MCP context, and portfolio rollups. It is OSS that runs in the adopter's CI; no data leaves the adopter's infrastructure by default.

## The problem Terrain solves

Modern AI/ML repos no longer have one test surface. They have unit tests, integration tests, eval suites, prompt templates, schemas, retrievers, datasets, pipelines, and generated artifacts. A change in one layer can invalidate protection in another, but most tools report only their own slice.

Terrain makes that protection graph visible before merge. It tells a team where validation is missing, which tests or evals a PR affects, and why a finding should block or warn in CI.

## Who it's for

| Role | What they get |
|---|---|
| **Frontend developer** | Learn before merge if a UI change breaks a downstream AI contract |
| **Backend / platform engineer** | Learn which models are affected by a schema or API change |
| **ML engineer (classical ML)** | Signals for test-set quality, drift/fairness preview coverage, train/test integrity, and data validation |
| **ML engineer (LLMs)** | Eval coverage, prompt/schema drift, retrieval quality, model-risk, and scenario-aware gate evidence |
| **Senior decision-maker** | Release status, limitations, supply-chain artifacts, and reproducible checks — auditable trust profile |

## What Terrain ships

- **Detection rules across ten categories** — regression, coverage, hygiene, reproducibility, security, performance, fairness, data, lifecycle, and documentation. Stable rules ship default-on when implemented, documented, and covered by tests; preview rules ship default-off while precision measurement and adopter feedback are still in progress.
- **Three surfaces** for the same diagnostic artifact: CI (status check + JUnit + GitHub annotations + Step Summary), CLI (`terrain test` / `terrain explain`), and an MCP server for AI assistants (Claude Code, Cursor).
- **Unified graph** that crosses file and format boundaries: schema↔prompt drift links a schema field to the prompt-template variable that references it (TS/JS ↔ Python/Go/Java), on top of schema-code-test relationships, database schema awareness (Postgres / MySQL / sqlc / gorm / prisma / sqlalchemy), and pipeline awareness (dbt / Airflow / Prefect). General cross-language API-spec edges (OpenAPI / tRPC / gRPC / GraphQL / HTTP-route inference) are planned, not yet shipped.
- **Trust-floor gate, default-on** — the `--fail-on` gate holds heuristic AI findings back from failing a build until Terrain can prove a fix, while failing tests, regressions, security/safety leaks, `policy.yaml` violations, and any Critical always gate. `terrain fix` applies the validated remediations (dry-run by default; `--apply` writes). Opt out with `--no-trust-floor`.
- **Multi-repo portfolio aggregation** via `terrain portfolio --from .terrain/repos.yaml`, with manifest-backed repo rollups, owner/tag propagation, snapshot-backed inputs, and framework-of-record drift detection.
- **Public quality artifacts:** feature-status, limitations, signed release artifacts, docs verification, and benchmark gates. Per-rule readiness cards are harness infrastructure and are not published unless a measured `harness/readiness/` card set is present for the release.
- **VS Code extension alpha** reading CLI JSON artifacts; renders findings in sidebar tree views with file reveal. Source and package metadata ship in the repo; Marketplace publication is future work.

## The trust profile

What Terrain commits to from the 0.2.0 stable-contract release onward:

| Surface | Contract |
|---|---|
| **Rule IDs** | Stable across minor versions; renames follow a one-cycle deprecation with stderr warnings |
| **JSON output schema** | `version: 1`; one-cycle deprecation cycle on changes |
| **`terrain.yaml` schema** | Versioned `v1`; closed enumeration for surface types; one-cycle deprecation on changes |
| **CLI flags** | Stable from 0.2.0; same deprecation contract |
| **Telemetry** | **No remote telemetry.** No phone home and no crash reports. Optional local-only telemetry writes to `~/.terrain/telemetry.jsonl` when explicitly enabled. Verifiable via `terrain --print-network` (which lists every external call Terrain would make under the current config — none for analysis) |
| **Data flow** | Templates tier: zero network calls. LLM provider config is parsed but inactive; external enrichment is future work and must remain explicit adopter choice when it ships |
| **Quality** | Release verification, docs consistency checks, fixture coverage, and benchmark gates run per release; per-rule readiness cards remain planned measured artifacts until generated |
| **License** | Apache 2.0 |

## What Terrain deliberately does *not* do

See `docs/LIMITATIONS.md` for the comprehensive list. The major items:

- **VS Code extension is alpha** — renders sidebar tree views from CLI JSON and supports file reveal; Marketplace publication, Problems-pane diagnostics, full LSP, JetBrains, and Neovim integration are future work
- **No cross-repo dependency graph edges** — `terrain portfolio --from` aggregates repo-level portfolio data, but gate decisions and code/eval dependency tracing remain per-repo
- **No observability-tool ingestion** — Honeycomb / Datadog / production-metric-based rules are not in scope today
- **No marketplace listings** — GitHub Marketplace Action, VS Code Marketplace extension, Claude Skill, and OpenAI listings are not yet published
- **Analyzed languages** — Go, JavaScript, TypeScript, Python, Java. Ruby, Rust, Kotlin, Swift, Scala, and C# are not analyzed

These boundaries do not reduce the core value proposition: Terrain is already a local, CI-ready pre-flight check for AI/ML test systems, with stable CLI/schema contracts, signed release artifacts, and documented integration paths.

## What adopting Terrain requires

| Step | Effort |
|---|---|
| Install Terrain (Homebrew / npm / `go install`) | Minutes |
| Run `terrain init` to scaffold `terrain.yaml` | Minutes |
| Run `terrain describe --write` to generate starter surface declarations (optional; can skip and write by hand) | 10–30 min one-time, depending on surface count |
| Wire `terrain test` into CI (GitHub Actions template provided; other CI can invoke the CLI and consume JUnit) | 10–30 min |
| Make `terrain/tests` a required branch-protection check (optional, recommended) | Minutes |
| Per stable rule that fires unexpectedly: triage as TP or FP, suppress via path-level ignore or severity downgrade in `terrain.yaml` | 60–120 sec per finding (target) |

No proprietary services to sign up for. Terrain itself makes no LLM provider calls; CI artifacts go only where the adopter's CI workflow publishes them.

## The maintenance commitment

What adopting Terrain costs the adopter team ongoing:

- **Per release of Terrain:** read the `CHANGELOG.md` for deprecations; rule-ID renames carry one-cycle warnings before removal.
- **Per failed gate:** triage the finding. Most stable rules target ≤60 sec; some categories target ≤90–180 sec for fix-direction.
- **Per false positive pattern:** path-level ignore or severity downgrade in `terrain.yaml`. If the FP is systematic, file a GitHub issue; the project commits to acknowledgment within 48 hours for stable-rule FP reports.
- **Per new surface (AI/ML component added to the codebase):** declare it in `terrain.yaml` `surfaces:` block. Adopters can run `terrain describe` to generate starter declarations, then edit descriptions by hand.

## What could break under the stability contract

Adopters should know:

- **Rule IDs are stable; their *detection precision* improves.** A rule that fires more accurately in a later release may flag findings on previously-clean code. This is documented in `CHANGELOG.md` but does not deprecation-cycle.
- **Preview rules can graduate to stable.** If an adopter opts into a preview rule and the project graduates it with a different default severity, the adopter's `terrain.yaml` override remains binding — the graduation doesn't change behavior in already-configured repos.
- **The supported renderer set is fixed** (`dorny/test-reporter`, `mikepenz/action-junit-report`, GitLab native). Other JUnit consumers may work but aren't part of the conformance contract.
- **The `terrain.yaml` `surfaces:` `type:` enum is closed.** Adding new types is a schema-version bump with a one-cycle deprecation. The 0.2.0 v1 set is 7 values (`llm | classical_ml | deep_learning | rag_pipeline | feature_pipeline | prediction_service | data_validator`).
- **Pre-0.2.0 was unstable by design.** There is no migration path from earlier versions; 0.2.0 is a clean slate. Adopters using earlier Terrain treat 0.2.0 as a fresh install.

## How to evaluate before adopting

An unfamiliar senior engineer / engineering manager / PM reading this overview, `docs/release/feature-status.md`, and `docs/LIMITATIONS.md` should be able to answer: (1) what category of tool is this? (2) what trust profile does it commit to? (3) what would adopting it require of my team? (4) what could break under the stability contract?

For a deeper read:
- `docs/PRODUCT.md` — product reference (mission, principles, scope, stability commitments)
- `docs/rules/<category>/<rule>.md` — per-rule documentation
- `docs/quickstart.md` — first report in five minutes
- `docs/cli-spec.md` — full command reference

## Where the category sits relative to neighbors

| Neighbor | What they do | What Terrain adds |
|---|---|---|
| Eval frameworks (promptfoo, deepeval, ragas, Great Expectations) | Produce verdicts on AI/ML quality | Surface those verdicts as native test cases in the CI gate; cross-stack cause attribution |
| Code-review bots (CodeRabbit, Greptile) | Generate narrative review comments | Failing tests with deterministic, structured diagnostics; not narrative |
| Impact-analysis platforms (Codecov, Bazel test-impact) | Code-to-test relationships | Same shape, extended to AI/ML evals; descriptive diagnostics for findings |
| Data-lineage tools (dbt lineage, OpenLineage) | Data flow | Cross-stack to AI surfaces; gate-blocking findings |
| MLOps platforms (MLflow, W&B, SageMaker) | Experiment tracking, model registry | Doesn't replace; integrates with as data sources |
| Static analysis (Semgrep, CodeQL) | Per-file code analysis | Graph-aware analysis spanning AI surfaces and evals |

Terrain sits in the gap between these — integrates with the eval frameworks and registries, emits its findings into the same CI test pipeline as the other test runners. It composes where possible and partially replaces where unavoidable (the AI-aware code-review-bot use case).

## Decision criteria

Adopt Terrain if:

- You have AI or ML in production and care about regressions
- You want failing tests as the gate signal, not bot comments
- You can run a Go binary in CI without adding a hosted service or LLM API key
- You're comfortable with a 0.x release (pre-1.0; stable APIs from 0.2.0 forward but the project is still maturing)

Hold off if:

- You need full IDE-integrated AI testing today (current VS Code alpha is sidebar tree views only)
- Your code lives in separate FE/BE repos and you need cross-repo cause attribution
- Your eval framework isn't in the supported list (promptfoo / deepeval / ragas / Great Expectations; gauntlet supported via JSON-format-compatible ingestion). You can adopt with one of those, or write a custom adapter and contribute it back.

---

*If you read this as part of an evaluation and felt anything was unclear or aspirational, please file a GitHub issue.*
