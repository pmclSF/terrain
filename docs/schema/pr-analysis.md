# PR Analysis Schema Contract

The canonical shape that `terrain report pr --json` emits and that
downstream tools (PR comment renderers, dashboards, CI scripts)
should parse against.

This is the audit-named gap (`pr_change_scoped.E4`) for "JSON shape
exists" — published here as a stable contract.

## Top-level envelope: `PRAnalysis`

```jsonc
{
  // PR analysis JSON schema version. Stability: Stable.
  "schemaVersion": "2",

  // Diff scope analyzed. Mirrors impact.ChangeScope.
  // Stability: Stable.
  "scope": {
    "baseRef": "main",
    "headRef": "HEAD",
    "changedFiles": [ "src/auth.go", "src/auth_test.go" ]
  },

  // One-sentence summary. Same wording as the headline of the
  // human-readable report.
  // Stability: Stable.
  "summary": "Mergeable — no new findings introduced.",

  // Change-risk posture band. One of:
  //   "well_protected" | "partially_protected" | "weakly_protected"
  //   "high_risk" | "evidence_limited"
  // Stability: Stable.
  "postureBand": "well_protected",

  // Per-area counts. Stability: Stable.
  "changedFileCount": 12,
  "changedTestCount": 3,
  "changedSourceCount": 9,
  "impactedUnitCount": 7,
  "protectionGapCount": 0,
  "totalTestCount": 472,

  // Findings introduced by THIS PR (not pre-existing debt).
  // Stability: Stable.
  "newFindings": [ /* ChangeScopedFinding, see below */ ],

  // Repository owners whose code is impacted by this PR.
  // Stability: Stable.
  "affectedOwners": [ "@platform-team" ],

  // Backward-compat: paths-only test list. Prefer testSelections
  // for new integrations. Stability: Stable (frozen for
  // back-compat; testSelections is the richer surface).
  "recommendedTests": [ "src/auth_test.go", "src/session_test.go" ],

  // Per-test selection with reasoning. Stability: Stable.
  "testSelections": [ /* TestSelection, see below */ ],

  // How the test set was chosen. One of:
  //   "direct-changes-only" | "direct+1-hop" | "full-impact"
  //   "explain-selection-rebuild"
  // Stability: Stable.
  "selectionStrategy": "direct+1-hop",

  // One-line reason this strategy was selected.
  // Stability: Stable.
  "selectionExplanation": "small change touching one module — direct + 1 hop covers the impact graph.",

  // Posture changes specific to the affected area.
  // Stability: Stable.
  "postureDelta": { /* PostureDelta */ },

  // Data gaps that limit the analysis (e.g. "no coverage data").
  // Stability: Stable.
  "limitations": [ "no runtime artifacts found — flake / slow detectors didn't run" ],

  // AI risk-review summary, when AI surfaces are detected.
  // Stability: Stable.
  "ai": { /* AIValidationSummary, see below */ }
}
```

The `impactResult` Go field is intentionally excluded from JSON
output (`json:"-"`) — it's a cross-package handle, not a stable
serialization shape. Use `terrain report impact --json` for the
full impact graph.

## `ChangeScopedFinding` — per-finding shape

```jsonc
{
  // Stable finding ID (round-trips via `terrain explain finding <id>`).
  // Stability: Stable.
  "findingId": "weakAssertion@src/auth_test.go:TestLogin#a1b2c3d4",

  // Detector type (one of the canonical signal types from
  // internal/signals/manifest.go). Stability: Stable.
  "type": "weakAssertion",

  // Severity. One of: critical | high | medium | low | info.
  // Stability: Stable.
  "severity": "high",

  // File path relative to repo root. Stability: Stable.
  "file": "src/auth_test.go",

  // Line where the issue was located, when known. Zero when not.
  // Stability: Stable.
  "line": 42,

  // Human-readable explanation. Stability: Stable.
  "explanation": "Assertion compares string to itself; check is meaningless.",

  // Plain-language reason this matters in the PR's context.
  // Stability: Stable.
  "whyItMatters": "Tests with self-comparing assertions don't catch regressions in the code they're meant to protect.",

  // Suggested fix or remediation. Stability: Stable.
  "suggestedAction": "Replace with a meaningful comparison or move the assertion.",

  // Whether this finding was introduced by THIS PR (true) or is
  // pre-existing debt touched by this PR (false). The `--new-findings-only`
  // gate uses this flag. Stability: Stable.
  "newInThisPR": true,

  // Pillar tag — currently always "gate" for findings emitted
  // here. Stability: Stable (per Track 2 pillar markers).
  "pillar": "gate"
}
```

## `TestSelection` — per-test reasoning

```jsonc
{
  // Test file or test ID. Stability: Stable.
  "test": "src/auth_test.go::TestLogin_RejectsExpiredToken",

  // Selection confidence. One of: high | medium | low.
  // Stability: Stable.
  "confidence": "high",

  // Whether this test directly exercises a changed code unit.
  // Stability: Stable.
  "isDirectlyChanged": true,

  // Per-reason chain — why this test was selected.
  // Stability: Stable.
  "reasons": [ { "reason": "covers AuthService.Login (changed)", "codeUnitId": "src/auth.go:Login" } ]
}
```

## `PostureDelta` — change-area posture shift

```jsonc
{
  // Bands before / after this PR's projected effect.
  // Stability: Stable.
  "before": "well_protected",
  "after":  "partially_protected",

  // Per-dimension changes. Stability: Stable.
  "dimensions": [
    { "name": "coverage_confidence", "before": "high", "after": "medium" }
  ]
}
```

## `AIValidationSummary`

```jsonc
{
  // AI capabilities affected by this change. Stability: Stable.
  "impactedCapabilities": [ "summarization", "rag" ],

  // Number of AI scenarios selected for this change.
  // Stability: Stable.
  "selectedScenarios": 5,

  // AI signals introduced by this PR — same shape as
  // ChangeScopedFinding above, AI-flavored. Stability: Stable.
  "blockingSignals": [ /* ChangeScopedFinding */ ],

  // AI gate verdict for the PR. One of: PASS | WARN | BLOCKED.
  // Stability: Stable.
  "gateVerdict": "PASS"
}
```

## Stability commitment

All fields named "Stability: Stable" are part of the long-lived
schema:
- New optional fields may be added in minor releases
- Removal requires a major version bump and a migration window
- Enum values may be extended in minor releases

The `pillar` field across `ChangeScopedFinding` is part of the
Track 2 pillar markers — it's always one of `understand`, `align`,
`gate`, or empty (`omitempty`).

## Versioning

`schemaVersion` follows the same convention as analysis.schema.json:
- **Patch** (1.0.x): doc-only changes
- **Minor** (1.x.0): new optional fields
- **Major** (x.0.0): breaking changes

## Consuming the JSON

```bash
# Pipe to jq for transformations:
terrain report pr --json | jq '.newFindings | length'

# Verdict gate: exit 0 unless there are new findings:
terrain report pr --json | jq -e '.newFindings | length == 0'

# All AI-flagged signals:
terrain report pr --json | jq '.ai.blockingSignals[]'

# Pillar grouping (with Track 2 pillar markers):
terrain report pr --json | jq '.newFindings | group_by(.pillar)'
```

## See also

- [`internal/changescope/model.go`](../../internal/changescope/model.go) — Go type definitions
- [`docs/schema/eval-adapters.md`](eval-adapters.md) — companion contract for AI eval ingestion
- [`docs/user-guides/pr-and-change-scoped-analysis.md`](../user-guides/pr-and-change-scoped-analysis.md) — end-user guide
- [`docs/examples/gate/github-action.yml`](../examples/gate/github-action.yml) — CI integration template
