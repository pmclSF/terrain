# Terrain for engineering managers

Terrain measures the *test system*, not individuals. The output is
designed to inform decisions like:

- Where should the next quarter's testing investment go?
- Which migrations are safe to start; which are blocked?
- Which areas of the codebase will hurt us if we ship without
  hardening tests there?
- Are we paying CI runtime / cost for tests that don't catch much?

What it deliberately doesn't do: rank engineers, attribute test debt
to authors, or produce per-person leaderboards.

## What you get

### Health grade

`terrain insights --json | jq '.healthGrade'` returns a single A / B / C
/ D grade for the repo's test suite, derived from the count of
Critical / High / Medium signals against the suite size. The rubric
lives at `docs/health-grade-rubric.md`; CI pipelines treat the grade
as a leading indicator, not a hard block.

### Top-3 recommendations

`terrain insights --root . --detail 1` produces a numbered list of
"the three things that would most improve this suite", ordered by
expected impact. Each recommendation cites the signals it draws from
so engineers can dig into the evidence.

### Risk posture

`terrain analyze --json | jq '.riskPosture'` returns a five-dimension
posture (reliability, change risk, governance, AI, structural) each
banded as Low / Medium / High / Critical. Useful for executive
summaries and roadmap reviews.

### Migration readiness

`terrain migration readiness --from <a> --to <b> --root .` answers
"can we move this codebase from framework A to framework B in this
quarter, or is there hidden work we're underestimating?" Output
includes a percentage estimate, a per-area breakdown, and the specific
blockers that would need to be addressed first.

## A typical workflow

```bash
# Quarterly review: what's the state of the test suite?
terrain insights --root . --detail 2

# Migration planning.
terrain migration readiness --from jest --to vitest --root .

# Risk profile for a release-readiness review.
terrain analyze --root . --json | jq '{healthGrade: .healthGrade, riskPosture: .riskPosture, weakAreas: .weakCoverageAreas}'
```

## Reading reports

Output is structured Headline → Findings → Profile → Next Actions.
Skim the headline; drill into Next Actions for delegation; share the
JSON snapshot with a team lead for follow-through.

A useful pattern for status reviews: capture `terrain analyze --json`
once a sprint into a stored artifact, then `terrain compare` two
snapshots to show progress over time.

## What Terrain does not do

- No per-developer attribution. Ownership data feeds routing
  ("this finding affects the auth team's code"), never scoring
  ("alice wrote 14 weak assertions").
- No external telemetry. Reports run locally; no data leaves your
  CI.
- No SaaS pricing. Terrain is OSS forever; the calibration corpus
  and per-detector confidence work serves all users.

## Where to go next

- `docs/health-grade-rubric.md` — what an A / B / C / D actually
  means.
- `docs/scoring-rubric.md` — how the risk posture is computed.
- `docs/release/0.2.md` — the active milestone roadmap.
