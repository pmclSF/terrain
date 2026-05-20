# Terrain Trust Ladder

How adopters move from "see what Terrain finds" to "block PRs on
what Terrain finds." Four rungs, each a clear next step. Don't skip
rungs — every team that does ends up with a noisy gate they can't
defend.

## Why a ladder

Terrain reports findings against the *current state* of your repo.
On day one, that state usually includes inherited debt: untested
exports a previous team shipped, AI surfaces without eval coverage,
mocks that crept in over years. A blocking gate against that
backlog brick CI on the first PR a contributor opens.

The ladder solves this by separating **visibility** from **gating**.
Each rung gives you more information; only the upper rungs block
merges.

## Rung 1 — Inventory

**What you do:** install Terrain locally, run `terrain analyze`, read
the report.

**What you get:** the test universe mapped — frameworks, test files,
code units, AI surfaces, eval scenarios, ownership, coverage gaps.
A baseline understanding of where your test system stands.

**What it doesn't do:** affect your CI. Run this rung locally for as
long as you want before promoting.

```bash
brew install pmclSF/terrain/mapterrain
cd your-repo
terrain analyze
```

**Move up when:** the report makes sense and you can describe
what it shows to a colleague.

## Rung 2 — Warnings

**What you do:** add the [recommended GitHub Action](../examples/gate/github-action.yml)
to your repo. Default config is warn-only.

**What you get:** every PR gets a Terrain comment with the
change-scoped risk report. Findings are visible in the PR review
flow; the build stays green.

**What it doesn't do:** block any merge. Surface things; let humans
decide.

```yaml
# In .github/workflows/terrain-pr.yml — copy from
# docs/examples/gate/github-action.yml
# (no --fail-on flag; warn-only is the default)
```

**Move up when:** the comments have run for two-to-four weeks and
the warning surface is calibrated — false positives are filed,
suppressions are in place, and the team agrees on which findings
should block.

## Rung 3 — CI annotations

**What you do:** add SARIF upload to the workflow (already in the
recommended template) so findings flow into the Security tab.
Optionally add `--format annotation` so PR-level annotations appear
in the diff view.

**What you get:** findings live in two places: the PR comment (high
signal, prose-shaped) and the Security tab (each finding addressable
by URL, navigable per-line in the diff). Reviewers can comment on a
specific Terrain finding the same way they comment on a CodeQL one.

**What it doesn't do:** block merges. SARIF surface is for review,
not enforcement.

**Move up when:** reviewers are routinely engaging with Terrain
findings in PRs and the team is ready to put a stake in the ground:
"from now on, no new HIGH-severity finding ships."

## Rung 4 — Blocking gates

**What you do:** flip on `--fail-on <severity>` in the workflow.
Pair with `--new-findings-only --baseline <path>` so existing debt
is grandfathered in.

**What you get:** the gate the platform team has been planning
for. CI fails on net-new findings at or above the chosen severity.
Suppressions remain the escape valve for legitimate waivers, with
required reasons + optional expiry.

```yaml
# Uncomment this line in the recommended template:
#   --fail-on critical
```

**Recommended pairing per pillar:**

- Severity gate: `--fail-on critical` (start) → `--fail-on high`
  (mature) → `--fail-on medium` (zero-tolerance branches)
- Baseline: `--new-findings-only --baseline
  .terrain/snapshots/latest.json`
- Policy: copy [`docs/policy/examples/balanced.yaml`](../policy/examples/balanced.yaml)
  to start; promote to [`strict.yaml`](../policy/examples/strict.yaml) over time
- Suppressions: `terrain suppress <finding-id> --reason "..." --expires
  YYYY-MM-DD --owner @platform` for legitimate waivers

**This rung is the destination.** Most teams settle here.

## What's NOT on the ladder

- **Hand-graded PR reviews of every Terrain finding.** Doesn't
  scale; the gate exists so reviewers can spend their time
  elsewhere.
- **Custom-thresholded per-team policy variants.** 0.2 ships
  one repo-wide policy. Per-team variants are tracked for 0.3.
- **Auto-fix of detected findings.** Terrain reports; humans fix.
  AST-grade auto-fix is on the long-term plan but explicitly not
  in 0.2 or 0.3.

## Common adoption mistakes

| Mistake | Why it fails | Fix |
|---------|--------------|-----|
| Jumping from Rung 1 to Rung 4 | Existing debt brick CI on day one | Pair with `--new-findings-only --baseline ...` always |
| Suppressions without `expires` | Waivers accumulate, audit becomes impossible | Default `--expires` to 90-180 days; renew or remove |
| Ignoring the AI Risk Review section | Heuristic detectors fire false positives in 0.2 — but signal-to-noise is good enough that ignoring is a real loss | Triage with `terrain explain finding <id>`; suppress or fix |
| Treating Tier-2 capabilities as Tier-1 | Marketing gets ahead of evidence; review scrutiny exposes it | Read `docs/release/feature-status.md` before claiming things publicly |

## Next reading

- [`docs/product/vision.md`](vision.md) — the product story behind
  the ladder
- [`docs/release/feature-status.md`](../release/feature-status.md) —
  per-capability tier so you know what's safe to lean on
- [`docs/policy/examples/`](../policy/examples/) — three starter
  policies matched to the ladder rungs
- [`docs/examples/gate/github-action.yml`](../examples/gate/github-action.yml) —
  the one recommended CI config
