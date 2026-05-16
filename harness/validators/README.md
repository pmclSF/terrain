# LB validators

One validator per LB quality bar. Each is a Go program/library that takes runner output and produces a per-rule pass/fail with evidence.

| File (TODO) | LB | What it checks |
|---|---|---|
| `schema_check.go` | LB-1 | Every emitted Finding validates against `schemas/finding.v1.json`. Doc page exists at `docs/rules/<rule-id>.md`. Worked example matches a real Finding from runner output. |
| `silence_check.go` | LB-3 | Across the labeled green corpus, zero PR comments / annotations / notifications. |
| `reproduction_parity.go` | LB-4 | CI-mode and CLI-mode Findings are byte-equivalent (modulo timestamps). |
| `fp_rate.go` | LB-5 | Per-rule Wilson 95% LB FP rate ≤ 5% on intended-green corpus. Min-sample-size floor N≥10. |
| `recall.go` | LB-6 | Per-rule recall ≥ 90% on seeded-failure corpus. |
| `junit_renderer_conformance.go` | LB-7 | JUnit XML renders cleanly in dorny/test-reporter, mikepenz/action-junit-report, GitLab native. |
| `runtime_budget.go` | LB-9 | Per-phase runtime within budget (graph ≤20s, rules ≤30s, render ≤5s, total ≤60s on 50k-file repo). |
| `fail_mode_probe.go` | LB-10 | Synthetic panic / OOM / I/O error → fail-closed by default. |
| `bidirectional_attribution.go` | LB-11 | FE→AI and prompt→FE scripted PRs both produce expected Findings with expected cause-paths. |

All status: TODO at 0.2.0 dev time. Implementation per Tier 4.
