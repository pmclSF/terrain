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

  // Owner from CODEOWNERS / .terrain/ownership.yaml.
  // Empty when no ownership data exists.
  // Stability: Stable.
  "owner": "@platform-team",

  // Number of test cases detected in this file.
  // Stability: Stable.
  "testCaseCount": 12,

  // Wall-clock runtime in milliseconds, when runtime artifacts
  // are ingested. Zero when no runtime data flows in.
  // Stability: Stable.
  "runtimeMs": 4520,

  // Structural coverage attributed to this test file in
  // [0.0, 1.0], when coverage artifacts are ingested.
  // Stability: Stable.
  "coverageRatio": 0.85,

  // Tags carried over from the .terrain/repos.yaml manifest
  // (e.g. ["tier-1", "customer-facing"]).
  // Stability: Stable.
  "tags": [ "tier-1" ]
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

  // Affected test file paths. Stability: Stable.
  "paths": [ "tests/auth/login_v1_test.go", "tests/auth/login_v2_test.go" ],

  // Confidence in the finding ([0.0, 1.0]).
  // Stability: Stable.
  "confidence": 0.78,

  // Severity classification. One of: critical | high | medium | low.
  // Stability: Stable.
  "severity": "medium",

  // Plain-language explanation. Stability: Stable.
  "explanation": "Both files exercise the same behavior surface (POST /login) with overlapping assertion sets.",

  // Recommended remediation. Stability: Stable.
  "suggestedAction": "Consolidate to a single test file or split coverage by precondition."
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
