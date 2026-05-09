# Portfolio Schema Contract

The canonical shape that `terrain portfolio` emits and that
multi-repo aggregator tooling parses against.

This is the audit-named gap (`portfolio.E4`) for "Schema for
portfolio output not documented" — published here as a stable
contract.

## Status

`terrain portfolio --from <manifest>` is **experimental** in 0.2.0
(Tier 3 in the capability map). The schema documented below is the
shape of the partial-shipping work in 0.2.0; multi-repo aggregation
matures in 0.2.x. Single-repo portfolio output is stable; the
multi-repo `--from <manifest>` shape may evolve before 0.3.

## Top-level: `PortfolioSummary`

```jsonc
{
  // Per-asset breakdown. One TestAsset per detected test file.
  // Stability: Stable (single-repo); Experimental (multi-repo).
  "assets": [ /* TestAsset, see below */ ],

  // Portfolio findings — redundancy candidates, overbroad tests,
  // low-value-high-cost, high-leverage. One entry per detected
  // pattern. Stability: Stable.
  "findings": [ /* Finding, see below */ ],

  // Summary statistics across the portfolio.
  // Stability: Stable.
  "aggregates": { /* PortfolioAggregates, see below */ }
}
```

## `TestAsset` — per-test-file record

```jsonc
{
  // Repo-relative path to the test file. Stability: Stable.
  "path": "tests/auth/login_test.go",

  // Detected framework name. Stability: Stable.
  "framework": "go-test",

  // Inferred test type (unit | integration | e2e). Stability: Stable.
  "testType": "unit",

  // Owner from CODEOWNERS / .terrain/ownership.yaml.
  // Empty when no ownership data exists.
  // Stability: Stable.
  "owner": "@platform-team",

  // Number of test cases detected in this file. Stability: Stable.
  "testCount": 12,

  // --- Cost metrics ---

  // Observed average runtime in milliseconds (0 if unknown).
  // Stability: Stable.
  "runtimeMs": 4520,

  // Observed retry rate in [0.0, 1.0]. Stability: Stable.
  "retryRate": 0.0,

  // Observed pass rate in [0.0, 1.0] (0 if unknown). Stability: Stable.
  "passRate": 0.998,

  // Inferred cost classification (one of the CostClass enum values).
  // Stability: Stable.
  "costClass": "moderate",

  // Count of health signals on this file (flaky, slow, etc.).
  // Stability: Stable.
  "instabilitySignals": 0,

  // --- Protection metrics ---

  // Number of code units this test covers. Stability: Stable.
  "coveredUnitCount": 8,

  // Set of directories/modules touched. Stability: Stable.
  "coveredModules": [ "internal/auth" ],

  // Number of exported code units covered. Stability: Stable.
  "exportedUnitsCovered": 5,

  // Set of distinct owners whose code is covered. Stability: Stable.
  "ownersCovered": [ "@platform-team" ],

  // Inferred breadth classification (one of the BreadthClass enum values).
  // Stability: Stable.
  "breadthClass": "focused",

  // Source files this test imports (from import graph). Used for
  // precise redundancy detection. Stability: Stable.
  "importedSources": [ "internal/auth/login.go" ],

  // --- Evidence ---

  // True if runtime data was available for cost estimation.
  // Stability: Stable.
  "hasRuntimeData": true,

  // True if coverage linkage was available for reach estimation.
  // Stability: Stable.
  "hasCoverageData": true
}
```

## `Finding` — per-detection record

```jsonc
{
  // Finding type. One of:
  //   "redundancy_candidate"  — file overlaps with another by
  //                             behavior surface
  //   "overbroad"             — file's runtime / coverage ratio
  //                             suggests it tests too much
  //   "low_value_high_cost"   — slow runtime + low coverage
  //   "high_leverage"         — fast + high coverage
  // Stability: Stable.
  "type": "redundancy_candidate",

  // Primary test file path for this finding. Stability: Stable.
  "path": "tests/auth/login_v1_test.go",

  // Other test file paths involved (e.g. for redundancy pairs).
  // Stability: Stable.
  "relatedPaths": [ "tests/auth/login_v2_test.go" ],

  // Resolved owner of the primary path. Stability: Stable.
  "owner": "@platform-team",

  // Confidence in the finding. String enum: "high" | "moderate" | "low".
  // Stability: Stable.
  "confidence": "high",

  // Plain-language explanation. Stability: Stable.
  "explanation": "Both files exercise the same behavior surface (POST /login) with overlapping assertion sets.",

  // Recommended remediation. Stability: Stable.
  "suggestedAction": "Consolidate to a single test file or split coverage by precondition.",

  // Type-specific detail. Stability: Stable.
  "metadata": { }
}
```

## `PortfolioAggregates` — summary stats

```jsonc
{
  // Total test files in the portfolio. Stability: Stable.
  "totalAssets": 472,

  // Sum of observed runtime in milliseconds. Zero when no
  // runtime artifacts flowed in. Stability: Stable.
  "totalRuntimeMs": 124300,

  // Share of total runtime consumed by the top 20% of tests
  // (Pareto concentration). Higher = more concentrated.
  // Stability: Stable.
  "runtimeConcentration": 0.62,

  // Whether any test in the portfolio has runtime data.
  // False means concentration / runtime-derived findings
  // are skipped. Stability: Stable.
  "hasRuntimeData": true,

  // Whether any test has coverage data. Stability: Stable.
  "hasCoverageData": true,

  // Per-finding-type counts. Stability: Stable.
  "redundancyCandidateCount": 12,
  "overbroadCount": 5,
  "lowValueHighCostCount": 8,
  "highLeverageCount": 23,

  // Per-owner aggregation. Stability: Stable.
  "byOwner": [
    {
      "owner": "@platform-team",
      "assetCount": 89,
      "totalRuntimeMs": 32100,
      "redundancyCandidateCount": 3,
      "overbroadCount": 1,
      "lowValueHighCostCount": 2,
      "highLeverageCount": 7
    }
  ]
}
```

## Multi-repo manifest contract: `.terrain/repos.yaml`

The companion manifest format consumed by
`terrain portfolio --from .terrain/repos.yaml`. Documented in
`internal/portfolio/manifest.go`'s `RepoManifest` Go type. The
canonical YAML shape:

```yaml
# Schema version. 0.2 ships v1.
version: 1

# Optional human-readable label for the manifest.
description: "Acme Corp engineering portfolio"

# Repos to aggregate over. At least one entry required.
repos:
  - name: web-app
    path: ../web-app
    owner: "@web-team"
    frameworksOfRecord: [jest]
    tags: [tier-1, customer-facing]

  - name: api-server
    snapshotPath: ../api-server/.terrain/snapshots/latest.json
    owner: "@platform-team"
    frameworksOfRecord: [go-test]
    tags: [tier-1]
```

Loader semantics in [`internal/portfolio/manifest.go`](../../internal/portfolio/manifest.go):

- **version** must be `1`. Unrecognized versions refuse-to-load
  (rather than guessing a degraded interpretation).
- **repos** cannot be empty. A manifest with zero repos is a load
  error — adopters need to know.
- Each `name` must be unique within a manifest.
- Each entry must have either `path` or `snapshotPath` set.
- `path` is relative to the manifest file's directory; the loader
  resolves it.

## Stability commitment

All fields named "Stability: Stable" above are part of the
long-lived schema for **single-repo portfolio output**. The
multi-repo aggregate output (`--from <manifest>`) is
**experimental** in 0.2.0 — its shape may evolve in 0.2.x as
the aggregator matures.

## Consuming the JSON

```bash
# Per-owner breakdown:
terrain portfolio --json | jq '.aggregates.byOwner[] | {owner, assetCount}'

# Top redundancy candidates:
terrain portfolio --json | jq '.findings[] | select(.type=="redundancy_candidate")'

# Tag-filtered roll-up (if tags are set in the manifest):
terrain portfolio --json | jq '.assets[] | select(.tags | contains(["tier-1"]))'
```

## See also

- [`internal/portfolio/manifest.go`](../../internal/portfolio/manifest.go) — Go type for the manifest
- [`internal/portfolio/model.go`](../../internal/portfolio/model.go) — Go types for the output shape
- [`docs/examples/align/multirepo/`](../examples/align/multirepo/) — runnable multi-repo example
- [`docs/schema/eval-adapters.md`](eval-adapters.md) — companion contract for AI eval ingestion
- [`docs/schema/pr-analysis.md`](pr-analysis.md) — companion contract for PR analysis
