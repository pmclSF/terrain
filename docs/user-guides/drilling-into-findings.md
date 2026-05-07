# Drilling into Findings

`terrain analyze` surfaces findings; `terrain insights`, `terrain
impact`, and `terrain explain` are the drill-down commands that take
you from a finding to its evidence and back.

This is the audit-named gap (`insights_impact_explain.P3`) for "how
confidence is computed" — published here as the drill-down playbook.

## The four commands and what each is for

| Command | Question it answers |
|---------|---------------------|
| `terrain analyze` | What's the state of the test system? |
| `terrain insights` | What should we fix in priority order? |
| `terrain impact` | What's affected by this change? |
| `terrain explain <target>` | Why did Terrain make this decision? |

## Drill-down ladder

Start with `analyze` and step down. Each command narrows scope.

### 1. `terrain analyze` — full snapshot

```bash
terrain analyze
```

Produces the full report: per-detector findings, posture by
dimension, test inventory, AI surface inventory. The right starting
point but too broad to act on directly.

### 2. `terrain insights` — prioritized recommendations

```bash
terrain insights
```

Re-renders the snapshot data as a ranked recommendation list:
"Health Grade: B; here are the 5 things to fix first." Each
recommendation includes a category (reliability / optimization /
architecture-debt / coverage-debt), a rationale, and an impact
estimate.

### 3. `terrain impact` — change-scoped analysis

```bash
# Default: HEAD~1 base
terrain impact

# Specific base ref (e.g. for PR review):
terrain impact --base main

# Per-test selection rationale:
terrain impact --explain-selection
```

Impact narrows the snapshot to "what's affected by this diff." The
output names changed code units, the tests that cover them, the
tests that *should* but don't, and a posture delta describing how
the change shifts the test-system state.

`--explain-selection` is the per-test reason chain: for every
recommended test, why was it selected? Adopters reviewing PRs
consult this when the recommendation surprises them.

### 4. `terrain explain <target>` — round-trip a finding to evidence

```bash
# Test file:
terrain explain src/auth_test.go

# Code unit:
terrain explain "src/auth.go:Login"

# Owner:
terrain explain "@platform-team"

# Stable finding ID (from JSON output or a previous explain):
terrain explain finding "weakAssertion@src/auth_test.go:TestLogin#a1b2c3d4"

# Selection (what tests would run for the current diff):
terrain explain selection
```

`explain` is the lowest level — it takes a single entity and
prints its evidence. Use this when a recommendation surprises you
and you want to see "why is this test flagged?"

## How confidence is computed

Most Terrain signals carry a confidence value in `[0.0, 1.0]`.
Here's where that number comes from:

### Detector confidence

Each detector emits a confidence reflecting how certain it is about
its judgment. Three kinds of detector:

1. **Structural-only detectors** (e.g. `weakAssertion`,
   `mockHeavyTest`): confidence is high (0.9–1.0) because the AST
   pattern is unambiguous.
2. **Heuristic detectors** (e.g. `aiToolWithoutSandbox`): confidence
   is medium (0.6–0.85) because they pattern-match in source code
   without dataflow analysis.
3. **Runtime-aware detectors** (e.g. `flakyTest`,
   `aiAccuracyRegression`): confidence depends on sample size —
   higher with more eval runs / more flake observations.

### Confidence intervals (`ConfidenceDetail`)

Some signals carry a `ConfidenceDetail` with a Wilson or Beta
interval (lower / upper bounds). Calibrated detectors emit this;
v1 detectors emit only the point estimate. See the SignalV2 fields
in `internal/models/signal.go`.

### Test-selection confidence

`terrain impact` and `terrain report select-tests` emit a
per-test `Confidence` field (`exact` / `inferred` / `weak`):

- **exact** — the test directly covers a changed code unit
- **inferred** — the test reaches the changed code transitively
  (1-hop or 2-hop in the import graph)
- **weak** — the test is in the same directory but no graph
  relationship exists

The confidence histogram in PR comments (above the recommended
tests table) summarizes the distribution at a glance.

### Coverage confidence

Per-file coverage attribution carries a confidence band
(`high` / `medium` / `low`) reflecting:
- whether coverage data was ingested (low without it)
- whether the test→code mapping is structural (high) or
  inferred from directory proximity (medium / low)

## Round-tripping a JSON finding

Every signal in `terrain analyze --json` carries a stable
`findingId`. That ID round-trips to evidence:

```bash
# Get a finding ID:
ID=$(terrain analyze --json | jq -r '.snapshot.signals[0].findingId')

# Round-trip it:
terrain explain finding "$ID"
```

The ID stays stable across runs as long as the underlying
`(Type, Location.File, Location.Symbol, Location.Line)` tuple
doesn't change. See `internal/identity/finding_id.go`.

## See also

- [`docs/schema/explain.md`](../schema/explain.md) — explain JSON contract
- [`docs/schema/pr-analysis.md`](../schema/pr-analysis.md) — PR + impact schema
- [`docs/schema/analysis.schema.json`](../schema/analysis.schema.json) — canonical snapshot shape
- [`docs/user-guides/explaining-posture.md`](explaining-posture.md) — posture-specific drill-downs
- [`docs/user-guides/impact-analysis-and-test-selection.md`](impact-analysis-and-test-selection.md) — deep dive on impact selection
- [`docs/user-guides/impact-drill-down-cli.md`](impact-drill-down-cli.md) — CLI flag reference for impact
