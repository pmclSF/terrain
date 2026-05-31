# Terrain — Architecture Overview

> **Pre-flight checks for AI/ML systems and the tests around them. Runs locally. No API key required.**

Terrain is a static analyzer that treats tests, evals, prompts, and schemas as one dependency graph — and catches the drift that crosses their boundaries before the PR merges.

The product story lives in [`docs/PRODUCT.md`](docs/PRODUCT.md). This document is the technical companion: what packages exist, what artifacts they produce, where the integration boundaries sit.

## Core principles

- **Signals are the core abstraction.** Every finding is a structured signal with type, severity, evidence, and location. See `internal/signals/manifest.go` for the manifest model.
- **The snapshot is the integration boundary.** `TestSuiteSnapshot` (`internal/models/snapshot.go`) is the serialized artifact at which detection, graph construction, impact analysis, and reporting compose. Anything that can serialize into the snapshot inherits graph traversal, impact analysis, and the diagnostic-rendering pipeline.
- **Risk must be explainable.** Risk surfaces are derived from signals with transparent scoring, not opaque scores.
- **Local-first, LLM-free by default.** Terrain analysis runs on a developer machine or CI runner with no accounts, SaaS, or network access required. No LLM API key is ever required. `terrain analyze` makes zero outbound network calls in the default configuration (verifiable via `terrain --print-network`). Install paths download signed binaries from GitHub Releases — that's the only network step.
- **Privacy boundary.** Aggregate metrics and benchmark exports never expose raw file paths or source code. Adopters with stringent code-confidentiality requirements can set `redact_source: true` in `terrain.yaml`.
- **Two-tier severity.** Detectors are explicitly classified as `gate` (counts toward `--fail-on=*` gate decisions) or `observability` (informational only). The tier is mandatory on every manifest entry — no implicit default.

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

Per the three-surface model in [`docs/PRODUCT.md`](docs/PRODUCT.md), Terrain has three consumer surfaces. All three consume the same artifact (JUnit XML + `findings.json` + the source repo state at the failing commit):

| Surface | Renderers | Interactivity | LLM |
|---|---|---|---|
| **CI** | JUnit XML, GitHub check-run annotations, Step Summary, status check | One-shot, passive | Never |
| **CLI** (`terrain analyze`, `terrain report explain`) | Terminal (cargo-style), JUnit, JSON | Developer-driven; supports follow-up questions | Optional (Ollama default, BYOK, or none) |
| **Agent** (MCP server) | MCP tool responses to Claude Code / Cursor / Apps SDK | Conversational | The agent's LLM (adopter's existing subscription) |

The artifact-as-handoff contract decouples surfaces. The CI surface ships at full quality without LLM features ever existing; the CLI and agent surfaces are additive enrichments.

## Package map

```
cmd/terrain/                  CLI entry point (adopter-facing)
cmd/internal/                 Maintainer-only tooling (not in adopter binary surface):
  terrain-bench/              Performance benchmarking
  terrain-bench-gate/         Benchmark regression gate (CI)
  terrain-convert-bench/      Conversion benchmark vs legacy reference
  terrain-docs-gen/           Doc stub generation from signal manifest
  terrain-docs-linkcheck/     Intra-repo markdown link check
  terrain-regression-gate/    Recall-regression gate
  terrain-truth-verify/       Manifest vs feature-status consistency check
  terrain-truthcheck/         Ground-truth fixture verifier
  terrain-voice-lint/         Voice and tone lint
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
  policy/                     Per-repo policy loader (enabled / disabled detector sets)
  quality/                    General test-quality detectors (mapped to hygiene/* and coverage/* rules at 0.2.0)
  reporting/                  Diagnostic format renderers (terminal, JUnit, GH annotations, Step Summary, SARIF)
  runtime/                    Test runtime artifact ingestion (JUnit, Jest/Vitest JSON)
  severity/                   Severity rubric and clause references
  signals/                    Signal manifest (rule registry; per-rule status: stable | experimental | planned)
  injection/                  Prompt-injection pattern library and test-input emitter
  scaffold/                   Mutation-test scaffold generator from JSON Schema
  plugin/                     Third-party plugin manifest schema and validator
  structural/                 Structural detectors (graph + AST joins)
  testcase/                   Per-language test extraction (AST + regex fallback)
  testtype/                   Test classification (unit / integration / e2e / component / smoke)
extension/vscode/             VS Code Marketplace extension (alpha at 0.2.0)
docs/                         Product, integration, rule, harness documentation
harness/                      Validation harness (corpora, runner, validators, readiness cards)
rfcs/                         RFCs for significant changes (governance per docs/CONTRIBUTING.md)
```

## CLI surface

The complete adopter-facing CLI is documented in [`docs/cli-spec.md`](docs/cli-spec.md). Summary:

| Command | Purpose |
|---|---|
| `terrain analyze [path]` | What is the state of our test system? Primary entry point. Writes `.terrain/findings.json` after every run. |
| `terrain test [flags]` | CI-mode wrapper around analyze. Emits JUnit XML and a markdown step-summary (point `--summary` at `$GITHUB_STEP_SUMMARY` for GitHub Actions). |
| `terrain report <verb>` | Read-side queries: `insights`, `impact`, `explain`, `summary`, `metrics`, `pr`, `posture`, `select-tests`. |
| `terrain migrate <verb>` | Framework conversion + migration workflow. |
| `terrain ai <verb>` | Eval scenarios: `list`, `run`, `doctor`, `record`, `baseline`, `baseline compare`, `replay`, `findings`. |
| `terrain inject --prompt <path>` | Generate jailbreak-shaped test inputs from a prompt template. |
| `terrain scaffold --schema <path>` | Generate boundary-case mutation tests from a JSON Schema. |
| `terrain plugins manifest <path>` | Validate a third-party plugin manifest. |
| `terrain config <verb>` | Workspace prefs: `feedback`, `telemetry`. |
| `terrain init [path]` | Set up Terrain in a repository. Writes `.terrain/policy.yaml` + an annotated `.terrain/policy.yaml.example`. |
| `terrain doctor [path]` | Diagnostics for current setup (registry, aliases, gitignore, per-rule policy overrides). |
| `terrain mcp [--root <dir>]` | Start the MCP server on stdio for AI assistants. Reads `.terrain/findings.json` from the last analyze run. |
| `terrain serve [flags]` | Local HTTP server with HTML report + JSON API. |
| `terrain portfolio [flags]` | Multi-repo workspace intelligence. |
| `terrain --print-network` | Audit: list every external call Terrain would make under current config. |

## Key design documents

| Document | Purpose |
|---|---|
| [`docs/PRODUCT.md`](docs/PRODUCT.md) | Canonical product plan (mission, goals, rule catalog, validation harness) |
| [`docs/OVERVIEW.md`](docs/OVERVIEW.md) | 1-pager for senior decision-makers evaluating Terrain for adoption |
| [`docs/LIMITATIONS.md`](docs/LIMITATIONS.md) | Honest list of what 0.2.0 does *not* do |
| [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) | RFC process, governance, rule lifecycle, issue triage |
| [`docs/rules/_template.md`](docs/rules/_template.md) | Canonical rule-page template |
| [`docs/integrations/_template.md`](docs/integrations/_template.md) | Canonical integration-doc template |
| [`SECURITY.md`](SECURITY.md) | Coordinated-disclosure policy |
| [`SECURITY-DATA-HANDLING.md`](SECURITY-DATA-HANDLING.md) | Data-flow doc for security review |
| [`rfcs/`](rfcs/) | RFCs for significant changes |
| [`docs/cli-spec.md`](docs/cli-spec.md) | Full CLI command and flag reference |
| [`docs/signal-catalog.md`](docs/signal-catalog.md) | Signal types, categories, and the four-tier data-availability model |

## Vocabulary

Canonical terms used in this doc and across `docs/PRODUCT.md`:

- **Surface** — point in the codebase where an AI/ML system is exposed (LLM call site, model inference endpoint, feature pipeline, prompt template, training script)
- **Eval** — oracle that produces a verdict on a surface's behavior
- **Metric** — score produced by an eval (rubric score, accuracy, F1, AUC, drift KL, latency, cost)
- **Finding** — single result from one rule, rendered to four surfaces
- **Rule** — configurable detection capability with stable ID (`terrain/<category>/<rule>`), severity default, doc page
- **Tier** — `gate` (counts toward `--fail-on=*`) or `observability` (informational). Mandatory on every manifest entry — no implicit default.
- **Cause path** — chain of graph nodes from a finding's primary location back to the change in the PR that caused it
- **Unified graph** — dependency graph spanning code, tests, surfaces, evals, data, and cross-language edges (bidirectional cause attribution)

## What's stable

Stable from 0.2.0 forward (see [`docs/PRODUCT.md`](docs/PRODUCT.md) Stability commitments):

- Rule IDs (`terrain/<category>/<rule>` namespace)
- JSON output schema (`version: 1` on `terrain report pr --json` and `findings.json`)
- `terrain.yaml` schema (versioned `v1`; closed-enumeration for surface types)
- CLI flags
- Artifact format (JUnit XML structure, SARIF for security rules, `findings.json` shape)
- Documented quality bars

All follow the one-cycle deprecation contract documented in [`docs/PRODUCT.md`](docs/PRODUCT.md) (Versioning).

## Migration context

Terrain originated as a multi-framework test converter. That migration surface lives in `internal/convert/` and the conversion-subsystem CLI (`migrate`, `convert`, `detect`, etc.). It ships in the same binary as the AI/ML CI gate, stable from 0.2.0 forward, positioned as a parallel product capability with its own narrative.

Pre-0.2.0 was unstable by design; 0.2.0 is the first release with stability commitments. Adopters on earlier versions treat 0.2.0 as a fresh install.

## Extension architecture

The VS Code extension is intentionally thin. It invokes Terrain's CLI, reads the artifact format (JUnit + `findings.json`), and renders views — no domain logic is duplicated in the extension. At 0.2.0 the extension ships as a Marketplace-published alpha with the minimum capability set documented in [`docs/cli-spec.md`](docs/cli-spec.md). Full LSP-based integration is future work.

See [`docs/vscode-extension.md`](docs/vscode-extension.md).
