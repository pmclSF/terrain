# Explain Schema Contract

`terrain explain <target>` returns a JSON shape that depends on the
target type. This document maps target → output shape so consumers
can build typed integrations against the explain surface.

This is the audit-named gap (`insights_impact_explain.E4`) for
"JSON shape exists" — published here as a stable contract.

## Target dispatch

`terrain explain <target> --json` emits one of the following shapes
based on what `<target>` resolves to:

| Target form | Output shape | Source schema |
|-------------|-------------|---------------|
| Path to a test file | `models.TestFile` | [analysis.schema.json](analysis.schema.json) |
| Symbol / fully-qualified test name | `models.TestCase` | [analysis.schema.json](analysis.schema.json) |
| Test ID (`path::name`) | `models.TestCase` | [analysis.schema.json](analysis.schema.json) |
| Code unit ID (`path:Name` or `path:Type.Method`) | `models.CodeUnit` | [analysis.schema.json](analysis.schema.json) |
| Owner string | `OwnerExplanation` (this doc) | below |
| Scenario ID | `aidetect.Scenario` | [internal/aidetect](../../internal/aidetect/) Go types |
| `selection` (literal) | `impact.ImpactResult` | [pr-analysis.md](pr-analysis.md) |
| Stable finding ID | `models.Signal` | [analysis.schema.json](analysis.schema.json) |
| Portfolio finding index | `models.Finding` | [portfolio.md](portfolio.md) |
| Signal type (e.g. `weakAssertion`) | first matching `models.Signal` | [analysis.schema.json](analysis.schema.json) |

When the target doesn't resolve, `terrain explain` exits with code 5
(not-found) and prints the canonical "accepted forms" list with a
"re-run analyze if the ID is from an older snapshot" hint. See
`internal/identity/finding_id.go` for the stable-ID format.

## `OwnerExplanation` — owner-target shape

When `<target>` is an owner string, the JSON output is:

```jsonc
{
  // Owner string from CODEOWNERS / .terrain/ownership.yaml.
  // Stability: Stable.
  "owner": "@platform-team",

  // Repo-relative paths owned by this owner. Stability: Stable.
  "ownedFiles": [ "src/auth.go", "src/session.go" ],

  // Test file paths covering the owned files. Stability: Stable.
  "testFiles": [ "tests/auth_test.go" ],

  // Total signals attributed to this owner's code. Stability: Stable.
  "signalCount": 7,

  // Top signals (capped at 10) with full Signal shape. Stability: Stable.
  "signals": [ /* models.Signal */ ]
}
```

## `terrain explain selection` — selection-target shape

The literal target `selection` returns the impact analysis for
the current diff. Same shape as `terrain impact --json` plus the
per-test reason chain that `--explain-selection` produces. See
[`pr-analysis.md`](pr-analysis.md) for the canonical PR / impact
contract.

## `terrain explain finding <id>` — finding-target shape

Two cases:

1. **Stable finding ID** (parses via `identity.ParseFindingID`).
   Output is a full `models.Signal`. The ID round-trips back to its
   evidence and a suggested suppression command.
2. **Numeric portfolio index or signal type**. Output is a
   `models.Finding` (portfolio) or `models.Signal` (snapshot).

When no entity matches, the error includes the three accepted
forms and a "re-run `terrain analyze` if the ID is stale" hint —
see `cmd/terrain/cmd_explain.go showFinding`.

## Stability commitment

Every shape this document references is Stable per the source
schema's tier annotations. The dispatch table above is itself
Stable — adding new target types (e.g. fixture targets in 0.3) is
an additive change.

## Consuming the JSON

```bash
# Explain a test file:
terrain explain src/auth_test.go --json | jq '.testCount'

# Round-trip a finding ID:
terrain explain finding "weakAssertion@src/auth_test.go:TestLogin#a1b2c3d4" --json \
  | jq '{type, severity, location: .location.file, suggestedAction}'

# Owner explanation:
terrain explain "@platform-team" --json | jq '{owner, signalCount}'

# Selection (current diff):
terrain explain selection --json | jq '.selectedTests | length'
```

## See also

- [`docs/schema/analysis.schema.json`](analysis.schema.json) — base snapshot shape (signals, test files, code units)
- [`docs/schema/pr-analysis.md`](pr-analysis.md) — PR + impact shape
- [`docs/schema/portfolio.md`](portfolio.md) — portfolio shape
- [`internal/identity/finding_id.go`](../../internal/identity/finding_id.go) — finding-ID grammar
- [`internal/explain/explain.go`](../../internal/explain/explain.go) — Go entry point
