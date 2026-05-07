# Writing a Terrain Policy

A Terrain policy is the file that turns observation into a CI gate.
`terrain analyze` tells you what's there; `terrain policy check`
asks whether what's there meets the rules you wrote.

This guide is the audit-named gap (`policy_governance.P3`) for how
to author one. Everything you need is in `.terrain/policy.yaml`.

## TL;DR

```bash
# 1. Scaffold a starter policy:
terrain init    # writes .terrain/policy.yaml when missing

# 2. Pick a starting template by stance:
cp docs/policy/examples/balanced.yaml .terrain/policy.yaml

# 3. Run policy check:
terrain policy check
```

`terrain init` writes a template; the three example files in
`docs/policy/examples/` (`minimal`, `balanced`, `strict`) are
opinionated starting points. Edit one to taste.

## Where the policy lives

A policy is a single file at `.terrain/policy.yaml` in the analyzed
repository. There is no central management, no DSL, no inheritance —
just a YAML file with a `rules:` block.

If the file doesn't exist, `terrain policy check` renders the
`EmptyNoPolicyFile` empty state ("Run `terrain init` to scaffold
.terrain/policy.yaml") and exits 0. The absence of a policy is not
itself a failure.

## Policy schema (the full surface)

```yaml
rules:
  # ── Test hygiene ──────────────────────────────────────────────

  # Block CI when any test is shipped with .skip / .only or
  # framework-equivalent. Catches the "skip pattern" anti-flow.
  disallow_skipped_tests: true

  # Block when any framework on this list is detected. Useful for
  # post-migration cleanup ("don't let karma sneak back in").
  disallow_frameworks:
    - karma
    - mocha-1.x

  # Block when average per-test runtime exceeds this (ms).
  max_test_runtime_ms: 5000

  # Block when structural coverage drops below this percent.
  minimum_coverage_percent: 70

  # Block when weakAssertion signal count exceeds N.
  max_weak_assertions: 10

  # Block when mockHeavyTest signal count exceeds N.
  max_mock_heavy_tests: 5

  # ── AI risk + gating ──────────────────────────────────────────

  ai:
    # Block on any safetyFailure signal (e.g. uncovered safety
    # eval on a safety-critical surface).
    block_on_safety_failure: true

    # Block when accuracy regresses by N percentage points vs
    # baseline. 0 = any regression blocks; 5 = block on >5 pp.
    block_on_accuracy_regression: 5

    # Block when a changed AI context surface has no scenario
    # coverage (the "you changed the system prompt and there's
    # nothing testing it" check).
    block_on_uncovered_context: true

    # Warn (don't block) on latency or cost regressions.
    warn_on_latency_regression: true
    warn_on_cost_regression: true

    # Warn when an AI capability has no eval coverage.
    warn_on_missing_capability_coverage: true

    # Custom block list — additional signal types that should
    # fail CI. Power tool: don't reach for it before the named
    # rules above are tuned.
    blocking_signal_types:
      - hallucinationDetected
      - aiPolicyViolation
```

Every rule is **opt-in**. A rule that's not present is not enforced.
Rules at the top level enforce on the analyzed repo's snapshot;
the `ai:` block enforces on AI risk-review signals specifically.

## Three opinionated starting points

Choose one and tune from there.

### `minimal.yaml` — observability only

Blocks on the absolute baseline (shipped skips, hard coverage floor
breach). Everything else is informational. Right when you're
adopting Terrain on a repo with significant existing debt and want
to see findings without bricking CI.

```yaml
rules:
  disallow_skipped_tests: true
  minimum_coverage_percent: 50
```

### `balanced.yaml` — recommended default

The everyday gate. Blocks on shipped skips, AI safety failures,
and meaningful accuracy regressions. Warns on cost / latency.
Pair with `--new-findings-only --baseline` so existing debt
doesn't brick day-one CI.

See [`docs/policy/examples/balanced.yaml`](../policy/examples/balanced.yaml).

### `strict.yaml` — tight feedback loops

Add to a healthy repo where the team wants Terrain enforcing
quality. Tighter thresholds on weak assertions, mock-heavy tests,
runtime budgets.

See [`docs/policy/examples/strict.yaml`](../policy/examples/strict.yaml).

## How the gate decides

`terrain policy check` evaluates every rule against the snapshot.
The result has three buckets:

- **PASS** — no rule violated; CI is green.
- **WARN** — rules in the `warn_on_*` family fired; informational
  but exit 0.
- **BLOCKED** — at least one block-class rule fired; exit 2
  (policy violation; same code as usage error pre-0.3 split).

The output renders the verdict in a hero block at the top of
`terrain policy check` output, with violations grouped by severity
underneath. See [`internal/reporting/policy_report.go`](../../internal/reporting/policy_report.go).

## Adopting in CI

The recommended pattern is "warn-only first, block second":

1. Add `terrain policy check` to CI as **non-blocking** for one
   week. Look at what fires.
2. Tune thresholds until the violations match your team's bar.
3. Promote `terrain policy check` to a blocking step.

The standard GitHub Action template at
[`docs/examples/gate/github-action.yml`](../examples/gate/github-action.yml)
makes both modes a one-line difference.

## Tuning rules: workflow

When a rule fires unexpectedly:

1. **Read the violation explanation.** Every violation includes
   `[SEV] type (Category) — explanation` plus a `location:` line.
   The explanation names which signal triggered.
2. **Drill into the signal:** `terrain explain finding <id>`
   round-trips a stable finding ID back to its evidence.
3. **Decide:** raise the threshold (you're tracking debt-down),
   suppress the specific finding (you've reviewed and accepted),
   or fix the underlying issue.
4. **For genuinely-acceptable findings, prefer suppressions over
   policy threshold changes** — suppressions document the *why*
   per finding, while threshold changes are blanket.

## Pairing with suppressions

`.terrain/suppressions.yaml` and `.terrain/policy.yaml` are
complementary:

- **Policy** — blanket rules ("no skipped tests", "min 70% coverage").
- **Suppressions** — per-finding waivers with reasons and expiry.

Suppressions ship in 0.2 (Track 4.5/4.6/4.7 — `terrain suppress
<id> --reason "<why>" --expires <date>`).

## What policy isn't

- **Not a DSL.** No conditionals, no logic. If you find yourself
  wanting "block when X but not Y", that's a sign the rule needs
  splitting at the detector level, not policy expressivity.
- **Not centralized.** Each repo owns its own policy file.
  Cross-repo policy aggregation is on the 0.3 roadmap (depends
  on multi-repo Track 6 maturing).
- **Not a security control.** Policy gates a CI build. It does
  not stop a determined developer from merging. Combine with
  branch protection rules.

## See also

- [`docs/policy/examples/`](../policy/examples/) — three starter policies
- [`internal/policy/config.go`](../../internal/policy/config.go) — full Go type definitions
- [`docs/user-guides/ai-eval-onboarding.md`](ai-eval-onboarding.md) — pair with AI rules
- [`docs/user-guides/getting-started.md`](getting-started.md) — install + first-run
