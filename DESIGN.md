# Terrain — Architecture Overview

Terrain is a CI gate for AI/ML systems. It treats unit tests, integration tests, e2e tests, and AI/ML evals as nodes in a single dependency graph and surfaces failures in that graph as native test cases in the platform's Tests tab — not as comments, dashboards, or emails.

The product story lives in [`docs/PRODUCT.md`](docs/PRODUCT.md). This document is the technical companion: what packages exist, what artifacts they produce, and where the integration boundaries sit.

## Core principles

- **Signals are the core abstraction.** Every finding is a structured signal with type, severity, evidence, and location. See `internal/signals/manifest.go` for the manifest model.
- **The snapshot is the integration boundary.** `TestSuiteSnapshot` (`internal/models/snapshot.go`) is the serialized artifact at which detection, graph construction, impact analysis, and reporting compose. Anything that can serialize into the snapshot inherits graph traversal, impact analysis, and the diagnostic-rendering pipeline. (Note: at 0.2.0, the impact-selection code paths consult the typed graph rather than direct-scanning `LinkedCodeUnits` — see Tier 0 work in `docs/PRODUCT.md` §16.)
- **Risk must be explainable.** Risk surfaces are derived from signals with transparent scoring, not opaque scores.
- **Local-first.** Terrain runs on a developer machine or CI runner with no accounts, SaaS, or network access required. The default configuration makes zero outbound network calls (verifiable via `terrain --print-network`).
- **Privacy boundary.** Aggregate metrics and benchmark exports never expose raw file paths or source code. Adopters with stringent code-confidentiality requirements can set `redact_source: true` in `terrain.yaml`.

## Pipeline

```
Repository scan
  → Framework detection + test-file discovery + AI/ML surface detection
    → Snapshot construction (TestSuiteSnapshot)
      → Dependency graph construction (depgraph) — typed nodes + edges with confidence
        → Impact analysis (impact) — change-scoped traversal of the unified graph
          → Rule evaluation — finding emission per the rule catalog
            → Diagnostic rendering (4 surfaces: terminal, JUnit XML, GitHub annotations, Step Summary)
              → Optional SARIF emission for security/* rules
                → CI status check + Tests tab population
```

The dependency graph is the integration boundary for adding detection capabilities. New detectors hook in at the snapshot layer, not at the rule layer.

## Three-surface model

Per `docs/PRODUCT.md` §7, Terrain has three consumer surfaces. All three consume the same artifact (JUnit XML + `findings.json` + the source repo state at the failing commit):

| Surface | Renderers | Interactivity | LLM |
|---|---|---|---|
| **CI** | JUnit XML, GitHub check-run annotations, Step Summary, status check | One-shot, passive | Never |
| **CLI** (`terrain test`, `terrain explain`) | Terminal (cargo-style), JUnit, JSON | Developer-driven; supports follow-up questions | Optional (Ollama default, BYOK, or none) |
| **Agent** (MCP server) | MCP tool responses to Claude Code / Cursor / Apps SDK | Conversational | The agent's LLM (adopter's existing subscription) |

The artifact-as-handoff contract decouples surfaces. The CI surface can ship at full quality without LLM features ever existing; the CLI and agent surfaces are additive enrichments.

## Package map

```
cmd/terrain/                  CLI entry point (adopter-facing)
cmd/terrain-corpus/           Maintainer-only: corpus management (extract, gate at 0.2.0)
cmd/terrain-precision/        Maintainer-only: detector precision benchmarking (score, compare at 0.2.0)
cmd/terrain-bench/            Performance benchmarking
internal/                     Core libraries (see internal/README.md for full listing)
  analysis/                   Repository scanning, framework detection, AI surface detection
  aidetect/                   AI/ML library and pattern detection (regex + AST)
  airun/                      Eval-framework adapter implementations
  changescope/                PR-scoped analysis
  convert/                    Test-framework conversion subsystem (parallel capability)
  depgraph/                   Typed dependency graph (21 node types, 18 edge types, forward+reverse traversal)
  engine/                     Pipeline orchestration
  impact/                     Change-scope analysis and impact propagation
  measurement/                Posture-band computation
  models/                     Canonical data types (TestSuiteSnapshot, CodeSurface, Eval, etc.)
  quality/                    General test-quality detectors (mapped to hygiene/* and coverage/* rules at 0.2.0)
  reporting/                  Diagnostic format renderers (terminal, JUnit, GH annotations, Step Summary, SARIF)
  runtime/                    Test runtime artifact ingestion (JUnit, Jest/Vitest JSON)
  severity/                   Severity rubric and clause references
  signals/                    Signal manifest (rule registry; per-rule status: stable | experimental | planned)
  testcase/                   Per-language test extraction (AST + regex fallback)
  testtype/                   Test classification (unit / integration / e2e / component / smoke)
extension/vscode/             VS Code Marketplace extension (alpha at 0.2.0)
docs/                         Product, integration, rule, harness documentation
harness/                      Validation harness (corpora, runner, validators, readiness cards)
rfcs/                         RFCs for significant changes (governance per docs/CONTRIBUTING.md)
```

## CLI surface (0.2.0)

The complete adopter-facing CLI is documented in `docs/PRODUCT.md` §16. Summary:

| Command | Purpose |
|---|---|
| `terrain test [path]` | Run analysis, emit findings (cargo-style terminal + JUnit + GH annotations + Step Summary) |
| `terrain test --json` | Stable JSON output (`version: 1`) |
| `terrain test --junit <path>` | JUnit XML to specified path |
| `terrain test --sarif <path>` | SARIF 2.1.0 for `security/*` findings |
| `terrain explain <rule>` | Narrative explanation (LLM-enriched if configured; template-only otherwise) |
| `terrain explain --from-run <id>` | Read from CI artifact, no local reproduction needed |
| `terrain describe` | Generate surface descriptions (LLM-mediated; first-run UX with security-friendly defaults) |
| `terrain accept-snapshot [id]` | Accept a baseline change (for `regression/snapshot-mismatch`) |
| `terrain init` | Initialize `terrain.yaml` with sensible defaults |
| `terrain --print-network` | Audit: list every external call Terrain would make under current config |

Plus the conversion subsystem's CLI (`convert`, `migrate`, `detect`, etc.) — a parallel capability sharing the AST infrastructure but with its own product narrative.

## Key design documents

| Document | Purpose |
|---|---|
| [`docs/PRODUCT.md`](docs/PRODUCT.md) | Canonical product plan (mission, goals, rule catalog, validation harness) |
| [`docs/OVERVIEW.md`](docs/OVERVIEW.md) | 1-pager for senior decision-makers evaluating Terrain for adoption |
| [`docs/LIMITATIONS.md`](docs/LIMITATIONS.md) | Honest list of what 0.2.0 does *not* do |
| [`docs/HARNESS.md`](docs/HARNESS.md) | Validation harness internals (corpora, validators, readiness cards) |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | RFC process, governance, rule lifecycle, issue triage |
| [`docs/rules/_template.md`](docs/rules/_template.md) | Canonical rule-page template |
| [`docs/integrations/_template.md`](docs/integrations/_template.md) | Canonical integration-doc template |
| [`SECURITY.md`](SECURITY.md) | Coordinated-disclosure policy |
| [`SECURITY-DATA-HANDLING.md`](SECURITY-DATA-HANDLING.md) | Data-flow doc for security review |
| [`rfcs/`](rfcs/) | RFCs for significant changes |
| [`docs/architecture.md`](docs/architecture.md) | Layered architecture detail |
| [`docs/signal-model.md`](docs/signal-model.md) | Signal abstraction and schema |
| [`docs/signal-catalog.md`](docs/signal-catalog.md) | Signal types and categories |
| [`docs/cli-spec.md`](docs/cli-spec.md) | Full CLI command and flag reference |

## Vocabulary

Canonical terms used in this doc and across `docs/PRODUCT.md`:

- **Surface** — point in the codebase where an AI/ML system is exposed (LLM call site, model inference endpoint, feature pipeline, prompt template, training script)
- **Eval** — oracle that produces a verdict on a surface's behavior. Vocabulary rename from earlier "scenario" usage is a Tier 0 must-ship item (`docs/PRODUCT.md` §16).
- **Metric** — score produced by an eval (rubric score, accuracy, F1, AUC, drift KL, latency, cost)
- **Finding** — single result from one rule, rendered to four surfaces
- **Rule** — configurable detection capability with stable ID (`terrain/<category>/<rule>`), severity default, doc page
- **Tier** — stable or preview (stable rules ship default-on at LB-measured quality; preview rules ship default-off as scope-under-evaluation)
- **Cause path** — chain of graph nodes from a finding's primary location back to the change in the PR that caused it
- **Unified graph** — dependency graph spanning code, tests, surfaces, evals, data, cross-language edges (LB-11 bidirectional)

## What's stable at 0.2.0

Per `docs/PRODUCT.md` §6 *Stable APIs from 0.2.0 release*:

- Rule IDs (`terrain/<category>/<rule>` namespace)
- JSON output schema (`version: 1` on `terrain pr --json` and `findings.json`)
- `terrain.yaml` schema (versioned `v1`; closed-enumeration for surface types)
- CLI flags
- Artifact format (JUnit XML structure, SARIF for security rules, `findings.json` shape)
- Documented LB quality bars (LB-1 through LB-12)

All follow the one-cycle deprecation contract per `docs/PRODUCT.md` §18 versioning.

## Migration context

Terrain originated as a multi-framework test converter. That migration surface lives in `internal/convert/` and the conversion-subsystem CLI (`convert`, `migrate`, `detect`, etc.). It ships in the same binary as the AI/ML CI gate at 0.2.0 and is stable from the 0.2.0 release tag, but is positioned as a parallel product capability with its own narrative (per `docs/PRODUCT.md` §7).

Pre-0.2.0 was unstable by design; 0.2.0 is the first release with stability commitments. Adopters using pre-0.2.0 versions treat 0.2.0 as a fresh install.

## Extension architecture

The VS Code extension is intentionally thin. It invokes Terrain's CLI, reads the artifact format (JUnit + `findings.json`), and renders views — no domain logic is duplicated in the extension. At 0.2.0 the extension ships as a Marketplace-published alpha with the minimum capability set documented in `docs/PRODUCT.md` §16. Full LSP-based integration lands in 0.3.0.

See [`docs/vscode-extension.md`](docs/vscode-extension.md).
