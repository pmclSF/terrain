# Terrain — Overview

> *For senior engineers, engineering managers, and product leaders evaluating Terrain for adoption. Read time: 5 minutes.*

## What Terrain is

**Terrain is a pre-flight check for AI/ML systems and the tests around them.** A static analyzer that inspects prompts, retrievers, classical-ML training pipelines, eval coverage, schema-code-test relationships, and the cross-language edges between them — in your PR, before merge. Locally, deterministically, on every push. **No API key required.**

Terrain treats unit tests, integration tests, e2e tests, and AI/ML evals as nodes in a single dependency graph. A frontend developer learns in their PR if their change will degrade a downstream AI model's behavior. An ML engineer learns which upstream code paths their prompt edit affects. The analysis spans code, prompt, schema, and eval boundaries.

It is **not** an eval framework — it integrates with promptfoo, deepeval, ragas, and Great Expectations. It is **not** a code-review bot — its output is failing test cases in the platform's Tests tab, not narrative comments on PRs. It is **not** a hosted service — it is OSS that runs in the adopter's CI; no data leaves the adopter's infrastructure by default.

## Who it's for

| Role | What they get |
|---|---|
| **Frontend developer** | Learn before merge if a UI change breaks a downstream AI contract |
| **Backend / platform engineer** | Learn which models are affected by a schema or API change |
| **ML engineer (classical ML)** | Regression detection on test-set metrics, drift, fairness; train/test integrity checks |
| **ML engineer (LLMs)** | Scenario-based eval regression detection across prompts, models, and RAG components |
| **Senior decision-maker** | Public per-rule readiness cards and reproducible benchmarks — auditable trust profile |

## What 0.2.0 ships

- **Detection rules across ten categories** — regression, coverage, hygiene, reproducibility, security, performance, fairness, data, lifecycle, and documentation. Stable rules ship default-on at measured quality; preview rules ship default-off and graduate as their false-positive rate and triage time clear the quality bars.
- **Three surfaces** for the same diagnostic artifact: CI (status check + JUnit + GitHub annotations + Step Summary), CLI (`terrain test` / `terrain explain`), and an MCP server for AI assistants (Claude Code, Cursor).
- **Unified graph** that crosses language boundaries: TS/JS ↔ Python/Go/Java via OpenAPI / tRPC / gRPC / GraphQL / HTTP-route inference, plus database schema awareness (Postgres / MySQL / sqlc / gorm / prisma / sqlalchemy) and pipeline awareness (dbt / Airflow / Prefect).
- **Public quality artifacts:** per-rule readiness cards (measured false-positive rate) published with every release.
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
| **Quality** | Load-bearing quality bars measured per release; readiness cards published per stable rule |
| **License** | Apache 2.0 |

## What 0.2.0 deliberately does *not* do

See `docs/LIMITATIONS.md` for the comprehensive list. The major items:

- **VS Code extension is alpha** — Marketplace-published, reads artifacts and renders findings in the Problems pane; full LSP / JetBrains / Neovim integration is future work
- **Single-repo analysis only** — adopters with FE/BE in separate repos run Terrain on each independently; cross-repo edges are future work
- **No observability-tool ingestion** — Honeycomb / Datadog / production-metric-based rules are not in scope today
- **No marketplace listings** — GitHub Action / Claude Skill / OpenAI listings are not yet published
- **Analyzed languages** — Go, JavaScript, TypeScript, Python, Java. Other languages (Ruby, Rust, Kotlin, Swift) are not analyzed today

## What adopting 0.2.0 requires

| Step | Effort |
|---|---|
| Install Terrain (Homebrew / npm / `go install`) | Minutes |
| Run `terrain init` to scaffold `terrain.yaml` | Minutes |
| Run `terrain describe --setup` to populate surface descriptions (optional; can skip and write by hand) | 10–30 min one-time, depending on surface count |
| Wire `terrain test` into CI (GitHub Actions or GitLab CI workflow snippet provided) | 10–30 min |
| Make `terrain/tests` a required branch-protection check (optional, recommended) | Minutes |
| Per stable rule that fires unexpectedly: triage as TP or FP, suppress via path-level ignore or severity downgrade in `terrain.yaml` | 60–120 sec per finding (target) |

No proprietary services to sign up for. No data leaves the adopter's infrastructure unless they explicitly enable an external-LLM tier.

## The maintenance commitment

What adopting Terrain costs the adopter team ongoing:

- **Per release of Terrain:** read the `CHANGELOG.md` for deprecations; rule-ID renames carry one-cycle warnings before removal.
- **Per failed gate:** triage the finding. Most stable rules target ≤60 sec; some categories target ≤90–180 sec for fix-direction.
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

An unfamiliar senior engineer / engineering manager / PM reading this overview, sample readiness cards, and `docs/LIMITATIONS.md` should be able to answer: (1) what category of tool is this? (2) what trust profile does it commit to? (3) what would adopting it require of my team? (4) what could break under the stability contract?

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

Adopt Terrain at 0.2.0 if:

- You have AI or ML in production and care about regressions
- You want failing tests as the gate signal, not bot comments
- You can run a Go binary in CI and (optionally) Ollama locally for diagnostic enrichment
- You're comfortable with a 0.x release (pre-1.0; stable APIs from 0.2.0 forward but the project is still maturing)

Hold off if:

- You need full IDE-integrated AI testing today (current VS Code alpha is Problems-pane only)
- Your code lives in separate FE/BE repos and you need cross-repo cause attribution
- Your eval framework isn't in the supported list (promptfoo / deepeval / ragas / Great Expectations; gauntlet supported via JSON-format-compatible ingestion). You can adopt with one of those, or write a custom adapter and contribute it back.

---

*If you read this as part of an evaluation and felt anything was unclear or aspirational, please file a GitHub issue.*
