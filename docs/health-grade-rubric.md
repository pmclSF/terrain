# Health Grade Rubric

`terrain insights` summarises an entire analysis with a single letter
grade — A, B, C, or D. This document is the canonical explanation of how
that grade is derived.

The companion document, `docs/scoring-rubric.md`, covers per-surface risk
band assignment.

## What the grades mean

| Grade | What it tells the user |
|---|---|
| **A** | Clean bill of health. No quality signals fired. |
| **B** | Minor issues. A handful of Low/Medium findings — ignorable in the short term, worth tracking. |
| **C** | Concerning. Either at least one High finding, or many Mediums. Schedule remediation before the next major release. |
| **D** | Failing. Either a Critical finding fired, or there are too many Highs to be a Medium incident. Treat as a release blocker until cleared. |

## The exact rule

`internal/insights/insights.go:deriveHealthGrade` evaluates these
clauses in order, returning the first match:

```
1. critical_findings > 0                           → D
2. high_findings    > healthGradeDHighFindingThreshold (3)   → D
3. high_findings    > 0                            → C
4. medium_findings  > healthGradeCMediumFindingThreshold (3) → C
5. medium_findings  > 0                            → B
6. any_finding      > 0                            → B
7. (no findings)                                   → A
```

Order matters: a snapshot with one Critical and zero of everything else
short-circuits on clause 1 and gets a D.

## What the constants actually mean

The two thresholds (`> 3` for both High → D and Medium → C) were chosen
during 0.1.0 design as round-number approximations of "more than a
handful." They are not calibrated against any external dataset; they are
a useful approximation that has held up across our internal sample of
~30 repos.

Specifically, "3" was chosen because:

- A team can plausibly investigate up to 3 High findings inside a single
  sprint. Beyond that, the team is firefighting and the grade should
  reflect that.
- The Medium threshold mirrors the High threshold for symmetry. Bumping
  one without the other tends to produce confusing edge cases.

Both thresholds are extracted as named constants
(`healthGradeDHighFindingThreshold`, `healthGradeCMediumFindingThreshold`)
in code so 0.3's calibration can adjust them in a single edit.

## Why grades are rule-based, not score-based

We considered computing the grade from the same risk-score model used
for risk surfaces (see `scoring-rubric.md`). We didn't, for three reasons:

1. **Grades are user-facing summaries.** Users intuitively understand
   "any Critical means D" — much more readily than "score ≥ 16 means D."
   The rule-based form maps directly onto the explanation an engineer
   gives a colleague.

2. **Counts compose better than sums for this use case.** Adding a
   Medium finding to a snapshot that already has 3 Highs shouldn't
   change the grade. The rule-based form makes that obvious; the
   score-based form requires careful threshold tuning to preserve it.

3. **Calibration of grades is independent.** Risk surfaces care about
   the relative balance of severities; grades care about whether
   thresholds were exceeded. Decoupling them lets each axis evolve
   without dragging the other along.

## What 0.3 changes

When the labelled corpus arrives:

- Threshold constants will be re-derived from corpus distribution
  rather than gut-feel.
- The grading function may incorporate weighted severity into clauses 2
  and 4 (e.g., "5 Highs of which all are policy violations" might
  produce D instead of C).
- The seven-clause cascade may collapse to a smaller table once we know
  empirically which boundaries matter.

We will not reduce the number of grades nor change their letter labels.
Public CI gates that key on `healthGrade == "D"` keep working through
the 0.3 transition.

## Edge cases

- **Empty repos** (zero findings, zero tests) grade as A. This is
  intentional: an empty repo has no quality issues, even if it also has
  no protection. Coverage problems for empty repos surface separately
  via `terrain insights → coverage` rather than collapsing into the
  grade.
- **Snapshots with only Info findings** grade as B, not A. Info-level
  observations document non-issues but still indicate the engine had
  something to say; A is reserved for "literally nothing fired."
- **Critical findings in `experimental` detectors** still produce a D.
  Severity is the contract; experimental status only affects whether a
  signal is enabled by default, not what its presence means.

## Reading this rubric in code

| Concept | Symbol | File |
|---|---|---|
| Grading function | `deriveHealthGrade` | `internal/insights/insights.go` |
| D-grade High count threshold | `healthGradeDHighFindingThreshold` | same |
| C-grade Medium count threshold | `healthGradeCMediumFindingThreshold` | same |

The grade also appears as the `healthGrade` field in `terrain insights
--json` output. That field is part of the snapshot v1 contract documented
at `docs/schema/COMPAT.md`, so it will not be removed or renamed without
a major version bump.
