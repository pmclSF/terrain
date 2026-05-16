# Terrain — Overview

> *For senior engineers, engineering managers, and product leaders evaluating Terrain for adoption. Read time: 5 minutes.*

## What Terrain is

Terrain is a CI gate for AI and ML systems. It treats unit tests, integration tests, e2e tests, and AI/ML evals as nodes in a single dependency graph, so a frontend developer learns in their PR if their change will degrade a downstream AI model's behavior — and an ML engineer learns which upstream code paths their prompt edit affects.

It is **not** an eval framework. It integrates with promptfoo, deepeval, ragas, and Great Expectations. It is **not** a code-review bot. Its output is failing test cases in the platform's Tests tab, not comments on PRs. It is **not** a hosted service. It is OSS that runs in the adopter's CI; no data leaves the adopter's infrastructure by default.

The category is *unified testing for AI/ML systems*. No comparable unified tool exists today; Terrain 0.2.0 is the reference implementation.

## Who it's for

| Role | What they get |
|---|---|
| **Frontend developer** | Learn before merge if a UI change breaks a downstream AI contract |
| **Backend / platform engineer** | Learn which models are affected by a schema or API change |
| **ML engineer (classical ML)** | Regression detection on test-set metrics, drift, fairness; train/test integrity checks |
| **ML engineer (LLMs)** | Scenario-based eval regression detection across prompts, models, and RAG components |
| **Senior decision-maker** | Public per-rule readiness cards, open calibration corpus, reproducible benchmarks — auditable trust profile |

## What 0.2.0 ships

- **75 detection rules** across 10 categories (30 stable, default-on; 45 preview, opt-in). Stable rules cover regression, coverage, hygiene, reproducibility, security, performance, and the classical-ML lifecycle.
- **Three surfaces** for the same diagnostic artifact: CI (status check + JUnit + GitHub annotations + Step Summary), CLI (`terrain test` / `terrain explain`), and an MCP server for AI assistants (Claude Code, Cursor).
- **Unified graph** that crosses language boundaries: TS/JS ↔ Python/Go/Java via OpenAPI / tRPC / gRPC / GraphQL / HTTP-route inference, plus database schema awareness (Postgres / MySQL / sqlc / gorm / prisma / sqlalchemy) and pipeline awareness (dbt / Airflow / Prefect).
- **Five dogfood reference repos** — three open-sourced as reference adopter setups (fullstack RAG; pure AI library; classical ML pipeline with dbt + Airflow + sklearn + MLflow).
- **Public quality artifacts:** per-rule readiness cards (measured FP rate via Wilson 95% lower bound; measured triage time via paid external panel) published with every release. Open-sourced labeled calibration corpus (CC-BY 4.0).
- **VS Code Marketplace extension** (alpha) reading the artifact format; renders findings in the IDE Problems pane.

## The trust profile

What Terrain commits to at the 0.2.0 release tag:

| Surface | Contract |
|---|---|
| **Rule IDs** | Stable across minor versions; renames follow a one-cycle deprecation with stderr warnings |
| **JSON output schema** | `version: 1`; one-cycle deprecation cycle on changes |
| **`terrain.yaml` schema** | Versioned `v1`; closed enumeration for surface types; one-cycle deprecation on changes |
| **CLI flags** | Stable from 0.2.0; same deprecation contract |
| **Telemetry** | **None by default.** No phone home, no usage stats, no crash reports unless opted in. Verifiable via `terrain --print-network` (which lists every external call Terrain would make under the current config — `none` for templates-only operation) |
| **Data flow** | Templates tier: zero network calls. Optional LLM tiers: Ollama (local; no data leaves machine) is the documented default; alternatives are explicit adopter choices |
| **Quality** | LB-1 through LB-12 measured per release on the dogfood repos; readiness cards published per stable rule |
| **License** | Apache 2.0 (Terrain itself); CC-BY 4.0 (labeled calibration corpus) |

## What 0.2.0 deliberately does *not* do

See `docs/LIMITATIONS.md` for the comprehensive list. The major items:

- **No IDE support beyond VS Code Marketplace alpha** — full LSP, JetBrains, Neovim land in 0.3.0
- **No polyrepo support** — adopters with FE/BE in separate repos run Terrain on each independently; cross-repo edges land in 0.4.0+
- **No observability-tool ingestion** — Honeycomb / Datadog / production-metric-based rules are 0.5.0+ work
- **No marketplace listings** — Marketplace Action / Claude Skill / OpenAI listings are 0.4.0+
- **5 supported languages** — Go, JS/TS, Python, Java. Ruby / Rust / Kotlin / Swift land in 0.3.0+

## What adopting 0.2.0 requires

| Step | Effort |
|---|---|
| Install Terrain (Homebrew / npm / `go install`) | Minutes |
| Run `terrain init` to scaffold `terrain.yaml` | Minutes |
| Run `terrain describe --setup` to populate surface descriptions (optional; can skip and write by hand) | 10–30 min one-time, depending on surface count |
| Wire `terrain test` into CI (GitHub Actions or GitLab CI workflow snippet provided) | 10–30 min |
| Make `terrain/tests` a required branch-protection check (optional, recommended) | Minutes |
| Per stable rule that fires unexpectedly: triage as TP or FP, suppress via path-level ignore or severity downgrade in `terrain.yaml` | 60–120 sec per finding (LB-2a/b targets) |

No proprietary services to sign up for. No data leaves the adopter's infrastructure unless they explicitly enable an external-LLM tier.

## The maintenance commitment

What adopting Terrain costs the adopter team ongoing:

- **Per release of Terrain:** read the `CHANGELOG.md` for deprecations; rule-ID renames carry one-cycle warnings before removal.
- **Per failed gate:** triage the finding. Most stable rules target ≤60 sec (LB-2a); some categories target ≤90–180 sec for fix-direction (LB-2b).
- **Per false positive pattern:** path-level ignore or severity downgrade in `terrain.yaml`. If the FP is systematic, file a GitHub issue; the project commits to acknowledgment within 48 hours for stable-rule FP reports.
- **Per new surface (AI/ML component added to the codebase):** declare it in `terrain.yaml` `surfaces:` block. Adopters can run `terrain describe` to auto-generate descriptions if they want narrative diagnostics.

## What could break under the stability contract

Adopters should know:

- **Rule IDs are stable; their *detection precision* improves.** A rule that fires more accurately in a later release may flag findings on previously-clean code. This is documented in `CHANGELOG.md` but does not deprecation-cycle.
- **Preview rules can graduate to stable.** If an adopter opts into a preview rule and the project graduates it with a different default severity, the adopter's `terrain.yaml` override remains binding — the graduation doesn't change behavior in already-configured repos.
- **The supported renderer set is fixed** (`dorny/test-reporter`, `mikepenz/action-junit-report`, GitLab native). Other JUnit consumers may work but aren't part of the conformance contract.
- **The `terrain.yaml` `surfaces:` `type:` enum is closed.** Adding new types is a schema-version bump with a one-cycle deprecation. The 0.2.0 v1 set is 7 values (`llm | classical_ml | deep_learning | rag_pipeline | feature_pipeline | prediction_service | data_validator`).
- **Pre-0.2.0 was unstable by design.** There is no migration path from earlier versions; 0.2.0 is a clean slate. Adopters using earlier Terrain treat 0.2.0 as a fresh install.

## How to evaluate before adopting

The plan commits to a senior-decision-maker evaluation bar (LB-12): an unfamiliar senior engineer / engineering manager / PM reading this overview, 3 sample readiness cards, and `docs/LIMITATIONS.md` should be able to answer (1) what category of tool is this? (2) what trust profile does it commit to? (3) what would adopting it require of my team? (4) what could break under the stability contract? Each release validates this with N=5 panelists; ≥4 must answer all four correctly before 0.2.0 ships.

For a deeper read:
- `docs/PRODUCT.md` — the canonical product plan (~1300 lines; for engineers planning the work)
- `docs/rules/<rule-id>.md` — per-rule documentation (75 pages at 0.2.0, all consistent with `docs/rules/_template.md`)
- `docs/HARNESS.md` — validation harness internals (how the LB measurements are produced)
- `docs/WALKTHROUGH.md` — worked end-to-end demo on a real adopter setup
- Per-release readiness cards — published in `harness/readiness/v0.2.0/<rule-id>.md` with each release tag

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

Adopt Terrain at 0.2.0 if:

- You have AI or ML in production and care about regressions
- You want failing tests as the gate signal, not bot comments
- You can run a Go binary in CI and (optionally) Ollama locally for diagnostic enrichment
- You're comfortable with a 0.x release (pre-1.0; stable APIs from 0.2.0 forward but the project is still maturing)

Hold off if:

- You need IDE-integrated AI testing today (wait for 0.3.0)
- Your code lives in separate FE/BE repos and you need cross-repo cause attribution (wait for 0.4.0+)
- You require Marketplace-distribution polish (the GitHub Marketplace listing ships in 0.4.0+)
- Your eval framework isn't in the supported list (promptfoo / deepeval / ragas / Great Expectations; gauntlet supported via JSON-format-compatible ingestion). You can adopt with one of those, or write a custom adapter and contribute it back.

---

*This document is validated by the LB-12 senior-decision-maker panel before each release. If you read this as part of an evaluation and felt anything was unclear or aspirational, please file a GitHub issue — that signal directly feeds the next release's rewrite.*
