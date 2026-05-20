# Validation Harness — internals

> *How Terrain's LB quality bars are measured and the per-rule readiness cards generated. The harness is what keeps the bars honest across releases.*

See `docs/PRODUCT.md` §13 for the harness's role in the product story. This document covers internals: components, protocols, where artifacts live, how to run the harness locally.

## Where it lives

The harness lives in the `harness/` directory of the Terrain repo:

```
harness/
├── README.md                       — pointers; reads like this doc minus the depth
├── triage-panel.md                 — recruitment, compensation, protocol details
├── corpora/
│   ├── terrain-testing-fullstack-rag/
│   │   └── labels.yaml             — hand-labeled PRs (intended-green + seeded-failure)
│   ├── terrain-testing-go-monolith/
│   │   └── labels.yaml
│   ├── terrain-testing-ai-only/
│   │   └── labels.yaml
│   ├── terrain-testing-polyglot-monorepo/
│   │   └── labels.yaml
│   └── terrain-testing-ml-pipeline/
│   │   └── labels.yaml
├── readiness/
│   └── v0.2.0/
│       ├── _summary.md             — overall release readiness summary
│       ├── regression-test-failed.md
│       ├── regression-eval-regression.md
│       └── ... (one per stable rule)
├── runner/                         — Go scripts that invoke Terrain on each PR
├── validators/                     — automated LB checks
└── reports/                        — per-release report bundles
```

## Components

### Test corpus

Each dogfood repo is pinned to a known SHA. Scripted PRs introduce specific violations — one per stable rule plus interaction-test cases. Lives in the dogfood repo's branch state, indexed by `harness/corpora/<repo>/labels.yaml`.

### Hand-labeled corpus

≥100 PRs per dogfood repo, hand-labeled in `labels.yaml`. Format:

```yaml
version: 1
labels:
  - pr: "PR#42 on terrain-testing-fullstack-rag"
    sha: <sha>
    expected_findings:
      - rule: regression/eval-regression
        eval: summarize_refusal
        ground_truth_cause: "frontend/CommentInput.tsx:42 — input length cap removed"
      - rule: coverage/no-tests
        unit: src/api/handlers/refund.py:RefundHandler.process
    rationale: "single-line example documenting why these findings are expected"
  - pr: "PR#43 on terrain-testing-fullstack-rag"
    sha: <sha>
    expected_findings: []  # intended-green
    rationale: "CSS-only change; no AI surfaces or impact"
```

Labeling effort: ~30 min per PR × 100 PRs × 5 repos ≈ 250 person-hours, single-labeler; ~500 person-hours with multi-labeler consensus via `terrain-corpus vote`.

### Runner

For each PR in the corpus, the runner:

1. Checks out the head SHA in a temporary clone
2. Invokes `terrain test --base <base_sha> --head <head_sha>` in CI mode
3. Invokes `terrain test --base <base_sha> --head <head_sha>` in CLI mode
4. Captures: JUnit XML, `findings.json`, Step Summary markdown, GH annotation payload, terminal output, exit code, wall-clock time, peak resident memory
5. Writes all outputs to `harness/reports/<release>/<repo>/<pr>/` for downstream validators

### Validators (automated)

Each validator implements one LB check and produces a per-rule pass/fail with evidence.

| Validator | LB | What it checks |
|---|---|---|
| `schema_check.go` | LB-1 | Every emitted Finding validates against the `Finding` JSON schema. Doc page exists at `docs/rules/<rule-id>.md`. Worked example in doc page matches a real `Finding` from runner output. |
| `silence_check.go` | LB-3 | For PRs labeled `expected_findings: []`, Terrain emits zero PR comments, zero inline annotations, zero notifications. Asserts on the status check being green and on no-side-effect emission. |
| `reproduction_parity.go` | LB-4 | For each Finding emitted in CI mode, the CLI-mode invocation produces a byte-equivalent Finding (modulo timestamps). Diffs the two JSON artifacts. |
| `fp_rate.go` | LB-5 | For each rule, computes Wilson 95% lower bound of FP rate on the intended-green corpus where the rule fired. Asserts ≤5%. Asserts minimum sample-size floor of N≥10 firings before binding the bar. |
| `recall.go` | LB-6 | For each rule, computes recall on the seeded-failure corpus where the rule was expected to fire. Asserts ≥90%. |
| `junit_renderer_conformance.go` | LB-7 | Runs Terrain's JUnit XML through `dorny/test-reporter`, `mikepenz/action-junit-report`, and GitLab's native JUnit consumer (via test fixtures). Asserts the rendered HTML contains expected test-case structure and failure messages. |
| `runtime_budget.go` | LB-9 | Measures cold-start, per-PR analysis time (graph / rules / render / total phases), peak resident memory on each dogfood repo. Asserts targets per §12 LB-9. |
| `fail_mode_probe.go` | LB-10 | Triggers a synthetic panic / OOM / I/O error in the harness; asserts the gate fails closed (status check red, clear annotation) under `on_terrain_error: block` and fails open under `on_terrain_error: pass`. |
| `bidirectional_attribution.go` | LB-11 | Runs the scripted FE→AI and prompt→FE PRs on `terrain-testing-fullstack-rag`; asserts both directions produce the expected `Finding` with the expected cause-path nodes. |

### Triage panel (manual)

External recurring panel of ~25 engineers, paid contractor model. See `harness/triage-panel.md` for recruitment, compensation, and per-rule protocol details.

Per release validation cycle:

1. Generate triage-panel session bundles: one PR per rule per dogfood repo where the rule fires, with the CI surface artifacts (no CLI / local checkout)
2. Each panelist sees N findings drawn from a stratified sample
3. For each finding, the panelist answers:
   - "Is this a true positive or false positive?" (LB-2a, decision in ≤60s P75)
   - If TP: "What's a candidate fix direction?" (LB-2b, articulation in ≤Ns P75 per category)
4. Independent grader compares panelist answers against the rule's ground-truth label (from `corpora/<repo>/labels.yaml`)
5. Per-rule report: median, P75, P90 triage time; correctness rate

Additional panel runs:
- **Agent-surface usability (LB-2c):** N=5 panelists use Claude Code with the MCP server installed; asked to resolve a real Terrain finding via the agent; success = correct fix direction within 10-min session
- **Senior-decision-maker comprehension (LB-12):** N=5 senior-level panelists read `docs/OVERVIEW.md` + 3 sample readiness cards + `docs/LIMITATIONS.md`; asked four questions about category, trust profile, adoption requirements, what could break

### Report generator

Once all validators (automated) and panel (manual) have produced outputs, the report generator assembles per-rule readiness cards.

Format per stable rule (committed to `harness/readiness/v<release>/<rule-id>.md`):

```markdown
# `terrain/regression/eval-regression` — v0.2.0 readiness card

| Bar | Target | Measured | Pass |
|---|---|---|---|
| LB-1 diagnostic completeness | doc page exists; worked example matches output | ✓ | ✓ |
| LB-2a triage decision P75 | ≤60s | 43s | ✓ |
| LB-2b fix-direction P75 (regression category) | ≤180s | 142s | ✓ |
| LB-4 reproduction parity | byte-equivalent CI ↔ CLI | ✓ | ✓ |
| LB-5 FP rate Wilson 95% LB | ≤5% per-repo | fullstack-rag: 2.1% (n=23) ✓; go-monolith: insufficient (n=2) ⚠ | partial |
| LB-6 recall on seeded-failure | ≥90% per-repo | fullstack-rag: 100% (n=5/5) ✓ | ✓ |
| LB-7 renderer conformance | dorny + mikepenz + GitLab | ✓ | ✓ |
| LB-9 runtime budget | ≤60s per-PR on 50k-file repo | 47s ✓ | ✓ |

Stable since: v0.2.0
Last validated: v0.2.0 (2026-XX-XX)
Panel session: 2026-XX-XX (N=5 panelists per repo)

Notes:
- LB-5 partial: rule did not fire frequently enough on terrain-testing-go-monolith for FP measurement to be statistically meaningful at the 5% bar (below n=10 floor). FP measurement on this repo is reported as "insufficient data" rather than a pass/fail.
```

Preview rules get partial cards (no LB-5 / LB-6 target binding, marked "preview — pending validation").

## Running the harness locally

```bash
cd harness/
go build -o run-harness ./runner
./run-harness --release v0.2.0 --repos all --validators all
./run-harness --release v0.2.0 --report
```

Validators run unattended; the triage panel session is a separate manual cycle coordinated through `harness/triage-panel.md`.

## Calibration corpus tooling (`terrain-corpus`, `terrain-precision`)

The harness depends on a labeled calibration corpus that drives per-detector precision floors. Two maintainer-only binaries manage it:

### `cmd/terrain-corpus`

At 0.2.0 ships two subcommands:

- `terrain-corpus extract` — pulls candidate findings from snapshots into the labeling pipeline
- `terrain-corpus gate` — enforces precision floors against the labeled corpus

The remaining subcommands (`sample`, `vote`, `adjudicate`, `aggregate`, `diff`, `regen`, `mine`) are active-learning and corpus-scaling infrastructure that 0.2.0's hand-labeled corpus doesn't need. They ship in 0.3.0+ when corpus scaling becomes necessary.

### `cmd/terrain-precision`

At 0.2.0 ships two subcommands:

- `terrain-precision score` — joins detector hits to ground-truth labels; computes per-(config, language, category) precision with Wilson-95% bounds
- `terrain-precision compare` — diffs precision metrics across configurations

`setup-fixtures` and `run` are full benchmark-harness orchestration that defers to 0.3.0+.

## Public release artifacts

Per the §3 *auditable quality* product goal, the harness's outputs ship as public artifacts:

1. **Per-rule readiness cards** committed to `harness/readiness/v<release>/<rule-id>.md` and published alongside the release tag.
2. **Open-sourced labeled calibration corpus** — `harness/corpora/*/labels.yaml` published under CC-BY 4.0 as a reference benchmark for the industry. Note: corpus contents (code snippets from real repos) may carry their original repos' licenses; data-provenance review precedes publication (§19 #6).
3. **Reproducible performance benchmarks** — per-dogfood-repo runtime numbers in the readiness summary, plus the harness scripts so adopters can run them themselves.

## How to add a new validator

When a new LB is added to `docs/PRODUCT.md` §12:

1. Define what the validator measures and what evidence it produces
2. Implement in `harness/validators/<name>.go`
3. Wire into the runner orchestration
4. Add a row to the readiness-card template
5. Re-validate all existing stable rules against the new bar (rules that no longer pass demote to preview honestly)

## How to update the labeled corpus

When a new dogfood repo is added or an existing one gains new failure modes:

1. Add PRs to the repo (scripted or organic)
2. Append entries to `harness/corpora/<repo>/labels.yaml` with `expected_findings:` per PR
3. Run `terrain-corpus extract` to pull candidate findings; review and label
4. Update `harness/readiness/` measurements on the next release cycle

---

*The harness is maintainer infrastructure. Adopters don't run it. Adopters consume its public outputs (readiness cards, performance benchmarks) when deciding whether to adopt or trust a specific rule.*
