# Scoring Rubric

This document is the canonical reference for how Terrain converts a snapshot
of signals into the **risk surfaces** users see in `terrain analyze`,
`terrain insights`, and `terrain explain`. It is the first half of a pair —
the second half (`docs/health-grade-rubric.md`) explains the per-report
A/B/C/D health grade.

The 0.1.2 release locks every magic number that affects scoring behind a
named constant in `internal/scoring/risk_engine.go`. This document explains
what each one means today and exactly what changes when 0.3's calibration
work lands.

## What the engine produces

For every analysed repository the risk engine emits a list of
`RiskSurface` entries. Each surface has:

- a **type** (`reliability`, `change`, `speed`, `governance`)
- a **scope** (`repository` or `directory`)
- a **band** (`low` / `medium` / `high` / `critical`)
- a numeric **score** that explains the band
- the list of **contributing signals** that fed the score
- an **explanation** string that quotes those numbers back to the user

Bands are categorical so users can reason about them without doing math.
Scores are exposed for power users and tooling that wants to compare
two snapshots quantitatively.

## Severity weights

Each contributing signal is weighted by its severity. The weights are
fixed for 0.1.x:

| Severity | Weight | Constant in code |
|---|---|---|
| Critical | 4.0 | `severityWeightCritical` |
| High | 3.0 | `severityWeightHigh` |
| Medium | 2.0 | `severityWeightMedium` |
| Low | 1.0 | `severityWeightLow` |
| Info | 0.5 | `severityWeightInfo` |

The relationship is **roughly linear** — one Critical equals two Mediums and
half a percentage of a High. These ratios were chosen by hand during 0.1.0
design so that "any Critical" dominates a band assignment regardless of
how many lower-severity findings are present. They are not corpus-derived,
and they will shift in 0.3 once we have ground-truth labels.

## How a score is computed

For each risk dimension the engine sums the severity-weights of every
signal in scope, producing `totalWeight`. It then computes both:

```
density  = (totalWeight / totalFiles) × 10
absolute = log(1 + totalWeight) × 1.2 + log(1 + signalCount) × 0.8
score    = max(density, absolute)
```

The `max` is intentional. Without it:

- A small repo with five Medium findings (totalWeight = 10, density = 10)
  would score "high" while a 1000-file repo with the same five findings
  (density = 0.1) would score "low" — even though the developer
  experience in both is equally bad.
- Conversely, a giant repo with thousands of trivial findings could
  produce a high density score that didn't reflect any concentration of
  risk.

Density captures concentration; absolute captures sheer burden; whichever
is worse drives the band. Both axes are independently tunable when 0.3
arrives.

## Band thresholds

```
score < 4               → low
4   ≤ score <  9        → medium
9   ≤ score < 16        → high
score ≥ 16              → critical
```

These four thresholds are uncalibrated. They were chosen during 0.1.0 to
produce three roughly evenly-sized bands across our internal sample of
~30 repos. 0.3 replaces them with corpus-percentile-derived values
calibrated against 50–100 labelled repositories; see
`docs/release/0.2.md` for the calibration plan and
`docs/release/feature-status.md` for the status of related work.

## Hysteresis

When `terrain compare` is used and the engine sees a previous band, it
applies a `±0.5` deadband around each threshold so an analysis that
hovers right at a boundary doesn't flap between two bands run-to-run.
The deadband only affects band assignment, not the score itself, and
only kicks in when a previous band is known. First-run analyses use the
plain `scoreToBand` mapping above.

## The governance floor

Governance violations are special-cased. If the governance dimension's
score lands below the Medium boundary AND a hard policy violation or a
Critical/High signal is present, the score is floored to 4.0. Without
this, a small repo with a single but real `policyViolation` would
otherwise emit a Low band — which would be technically correct given the
math but materially misleading given the meaning. The floor is the only
case where the band is not a pure function of the score; it is documented
inline in `risk_engine.go` and tested in `risk_engine_test.go`.

## Why these numbers, today?

Short answer: they were carried forward from 0.1.0 because changing them
is a behaviour-breaking event for every customer that has tuned policy
gates around current band assignments. 0.1.2's job is to make the
existing behaviour transparent and inspectable, not to replace the model.

Long answer:

1. The model was designed to be **explainable** end-to-end. Every
   constant is named, every formula is one line of code, and every band
   assignment can be traced back to a list of signals.
2. The values were **internal-corpus heuristics**. We ran them against
   a representative sample of repositories, eyeballed where the
   boundaries should land, and locked them.
3. We have always known **calibration is needed**. The plan since 0.1.0
   has been to land it once we had a labelled corpus large enough to
   resist over-fitting. That arrives in 0.3.

## What 0.3 changes

When the calibration corpus lands:

- Severity weights become whatever maximises labelled-repo precision/recall.
- Band thresholds become corpus percentiles (e.g., the 75th-percentile
  score across the corpus might become the Medium/High boundary).
- The hybrid `max(density, absolute)` formula is re-evaluated against
  the corpus; one or both axes may be dropped or replaced.
- Every numeric value gets a confidence interval reported in
  `terrain explain`.

The migration plan is to ship the new model behind `--scoring=v3` for one
release, give consumers time to recalibrate their CI gates, then make it
default. Bands and band names are stable; only the math underneath
changes.

## Reading this rubric in code

| Concept | Constant | File |
|---|---|---|
| Severity weight | `severityWeight*` | `internal/scoring/risk_engine.go` |
| Band thresholds | `riskBand*Upper` | same |
| Hysteresis deadband | `riskBandHysteresis` | same |
| Density multiplier | `densityScoreScale` | same |
| Absolute formula scales | `absoluteWeightScale`, `absoluteCountScale` | same |
| Governance floor | `governanceFloorScore` | same |
| Health grade thresholds | `healthGrade*Threshold` | `internal/insights/insights.go` |

If you change any of them, document the rationale in this file and update
the relevant boundary tests. Round 4 review pinned the failure mode that
allowed magic numbers to drift unchecked; the named constants exist
specifically so this stays auditable.
