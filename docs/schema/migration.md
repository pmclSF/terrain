# Migration Schema Contract

The canonical shapes that the `terrain migrate *` namespace emits as
JSON. Adopters scripting migrations against `terrain migrate
estimate --json` and `terrain migrate run --report-json` should
parse against these contracts.

This is the audit-named gap (`migration_conversion.E4`) for the
per-direction shapes — published here as a stable reference.

## Status

The migration namespace is **stable** for the Tier-1 conversion
directions (Jest ↔ Vitest is the canonical example). Other
directions are tagged Experimental in `terrain migrate list`
output; their schema remains the same but conversion confidence
ratings are still being calibrated. See
[`docs/release/feature-status.md`](../release/feature-status.md)
for the current per-direction tier matrix.

## `terrain migrate estimate --json` — `MigrationEstimate`

```jsonc
{
  // Repo root for the estimation. Stability: Stable.
  "root": "/path/to/repo",

  // Source framework (e.g. "jest"). Stability: Stable.
  "from": "jest",

  // Target framework (e.g. "vitest"). Stability: Stable.
  "to": "vitest",

  // Summary counts. Stability: Stable.
  "summary": {
    "totalFiles": 42,
    "testFiles": 28,
    "helperFiles": 6,
    "configFiles": 3,
    "otherFiles": 5,
    "predictedHigh": 22,
    "predictedMedium": 4,
    "predictedLow": 2
  },

  // Per-file records. Stability: Stable.
  "files": [ /* MigrationFileRecord, see below */ ],

  // Top conversion-blocking patterns detected.
  // Stability: Stable.
  "blockers": [
    {
      "pattern": "jest.mock(...)",
      "count": 8,
      "impact": "Manual review needed for module-level mocks."
    }
  ],

  // Effort estimate. Stability: Stable.
  "estimatedEffort": {
    "lowConfidenceFiles": 2,
    "mediumConfidenceFiles": 4,
    "estimatedManualMinutes": 95,
    "description": "~1.5 hours of manual review on top of automated conversion."
  }
}
```

## `MigrationFileRecord` — per-file detail

```jsonc
{
  // Repo-relative input path. Stability: Stable.
  "inputPath": "tests/auth/login.test.js",

  // File classification: "test" | "helper" | "config" | "other".
  // Stability: Stable.
  "type": "test",

  // Confidence score in [0, 100]. ≥90 = high, 70–89 = medium,
  // <70 = low. Stability: Stable.
  "confidence": 92,

  // Per-file rationale (why this confidence). Stability: Stable.
  "rationale": "Standard Jest test shape; conversion rules cover every assertion."
}
```

## `terrain migrate run --report-json` — `MigrationResult`

```jsonc
{
  "root":   "/path/to/repo",
  "from":   "jest",
  "to":     "vitest",

  // Output directory if --output was set. Empty = in-place.
  "output": "converted/",

  // Per-file conversion outcomes. Stability: Stable.
  "processed": [
    {
      "inputPath": "tests/auth/login.test.js",
      "type": "test",
      "confidence": 92,
      "rationale": "..."
    }
  ],

  // Optional checklist text emitted by the converter.
  // Stability: Stable.
  "checklist": "...",

  // State summary. Stability: Stable.
  "state": { /* MigrationStatus */ }
}
```

## `MigrationStatus` — run aggregation

```jsonc
{
  "total":      28,    // total candidates
  "converted":  24,    // succeeded
  "failed":     2,     // converter raised an error
  "skipped":    1,     // intentionally skipped (low confidence + no --force)
  "pending":    1,     // not yet processed
  "source":     "jest",
  "target":     "vitest",
  "startedAt":  "2026-05-04T12:00:00Z",
  "updatedAt":  "2026-05-04T12:08:33Z",
  "outputRoot": "converted/"
}
```

## `terrain migrate doctor --json` — `MigrationDoctorResult`

```jsonc
{
  "checks": [
    {
      "id":     "git-history",
      "label":  "Git history",
      "status": "pass",       // pass | warn | fail
      "detail": "Repo has > 10 commits; baseline-comparison ready.",
      "verbose": "...",        // verbose-only details
      "remediation": "..."     // present when status != pass
    }
  ],
  "summary": {
    "pass":  4,
    "warn":  1,
    "fail":  0,
    "total": 5
  },
  "hasFail": false,
  "pillars": [ /* Pillar status from cmd_doctor_pillars.go */ ]
}
```

The `pillars` field arrived in Track 2 (`PR #167`) and adds
per-pillar maturity assessment alongside the legacy migration
checks.

## Stability commitment

Every field named "Stability: Stable" is part of the long-lived
schema:

- New optional fields may be added in minor releases
- Removal requires a major version bump and a migration window
- Confidence-score thresholds (90/70) may shift in 0.3 once the
  conversion-corpus calibration work lands; the field shape itself
  stays stable.

## Per-direction status

`terrain migrate list --json` returns a table of supported
conversion directions. Each row carries a `tier` field:

```jsonc
{
  "directions": [
    {
      "from":          "jest",
      "to":            "vitest",
      "tier":          "stable",       // stable | experimental | preview
      "description":   "...",
      "calibrationStatus": "..."
    }
  ]
}
```

The `tier` field follows the Track 6.6 vocabulary documented in
[`docs/release/feature-status.md`](../release/feature-status.md).
Experimental and preview directions emit a banner-warning when
invoked via `terrain migrate run` (see `cmd/terrain/cmd_workflow.go`).

## Consuming the JSON

```bash
# Pre-flight risk assessment:
terrain migrate estimate --from jest --to vitest --json \
  | jq '{summary, blockerCount: (.blockers | length)}'

# Per-file confidence histogram:
terrain migrate estimate --from jest --to vitest --json \
  | jq -r '.files[] | "\(.confidence) \(.inputPath)"' | sort -n

# Failed-only review after a run:
terrain migrate status --json | jq '.processed[] | select(.status=="failed")'
```

## See also

- [`internal/convert/workflow.go`](../../internal/convert/workflow.go) — Go type definitions
- [`docs/user-guides/migrating-test-frameworks.md`](../user-guides/migrating-test-frameworks.md) — adopter-facing guide (when present)
- [`docs/release/feature-status.md`](../release/feature-status.md) — per-direction tier table
- [`docs/schema/portfolio.md`](portfolio.md) — companion contract for portfolio output
- [`docs/schema/eval-adapters.md`](eval-adapters.md) — companion contract for AI eval ingestion
