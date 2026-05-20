#!/usr/bin/env python3
"""
Phase A.5 — Severity-demote null hypothesis.

Question: if we demote the 7 lowest-precision detectors to observability tier,
what does the gate-tier panel's aggregate precision look like? And what's the
volume change in findings the user sees?

Inputs:
  tier-4/detector-validation.jsonl        (n=50)
  tier-4/detector-validation-n200.jsonl   (disjoint n=200)

Output:
  tier-4/phase-a5-results.json + stdout summary table

Decision: if gate-tier panel aggregate precision after demotion is >= 85%,
the entire "build A1-A5 infrastructure" plan is unnecessary for cycle 1.
"""

from __future__ import annotations
import json
import math
import sys
from collections import Counter, defaultdict
from pathlib import Path


# Detectors proposed for demotion to observability (precision <40% at merged n=250)
# uncoveredAISurface stays gate-tier (moat — Phase C #4 fixes with proximity classifier)
DEMOTION_CANDIDATES = {
    "migrationBlocker",        # 0% — capability preserved as observability
    "testsOnlyMocks",          # 0.7% — base rate too low for gate
    "assertionFreeImport",     # 17.3% — needs A1 (regex floor)
    "weakAssertion",           # 20.5% — needs A1 (regex floor)
    "deprecatedTestPattern",   # 21.0% — refresh trigger set, observability first
    "assertionFreeTest",       # 35.6% — needs A1 (regex floor)
    "orphanedTestFile",        # 35.8% — needs A2 (import-graph); aggregate stays gate
}


def load_merged() -> list[dict]:
    """Load and dedupe rows from both n=50 and n=200 jsonl files."""
    rows = []
    for path in [
        "tier-4/detector-validation.jsonl",
        "tier-4/detector-validation-n200.jsonl",
    ]:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    rows.append(json.loads(line))
                except json.JSONDecodeError:
                    continue
    # Dedupe on (repo, rule_id, file, symbol)
    seen = set()
    deduped = []
    for r in rows:
        k = (r.get("repo", ""), r.get("rule_id", ""),
             r.get("file", ""), r.get("symbol", ""))
        if k in seen:
            continue
        seen.add(k)
        deduped.append(r)
    return deduped


def wilson_lcb(tp: int, n: int, z: float = 1.96) -> float:
    """Wilson 95% lower confidence bound for precision."""
    if n == 0:
        return 0.0
    p = tp / n
    denom = 1 + z * z / n
    centre = p + z * z / (2 * n)
    spread = z * math.sqrt((p * (1 - p) + z * z / (4 * n)) / n)
    return max(0.0, (centre - spread) / denom)


def bonferroni_lcb(tp: int, n: int, k: int = 17) -> float:
    """Bonferroni-adjusted LCB across k simultaneous decisions."""
    # alpha=0.05 -> per-test alpha=0.05/k -> z for two-sided
    # Just use z=2.97 as approximation for k=17 (per L1 review)
    return wilson_lcb(tp, n, z=2.97)


def main():
    rows = load_merged()
    print(f"loaded {len(rows)} deduped rows from merged n=250", file=sys.stderr)

    # Per-detector counts
    by_det = defaultdict(lambda: {"TP": 0, "FP": 0, "UNC": 0, "OTHER": 0})
    for r in rows:
        d = r.get("rule_id", "?")
        v = (r.get("_verdict") or {}).get("verdict", "?")
        if v in by_det[d]:
            by_det[d][v] += 1
        else:
            by_det[d]["OTHER"] += 1

    # Scenario 1: All detectors gate-tier (current state)
    s1_tp = sum(c["TP"] for c in by_det.values())
    s1_fp = sum(c["FP"] for c in by_det.values())
    s1_n = s1_tp + s1_fp
    s1_findings = sum(c["TP"] + c["FP"] + c["UNC"] for c in by_det.values())
    s1_prec = s1_tp / max(1, s1_n) * 100
    s1_lcb = wilson_lcb(s1_tp, s1_n) * 100
    s1_lcb_bonf = bonferroni_lcb(s1_tp, s1_n) * 100

    # Scenario 2: 7 worst demoted to observability
    gate_dets = [d for d in by_det if d not in DEMOTION_CANDIDATES]
    s2_tp = sum(by_det[d]["TP"] for d in gate_dets)
    s2_fp = sum(by_det[d]["FP"] for d in gate_dets)
    s2_n = s2_tp + s2_fp
    s2_findings_gate = sum(by_det[d]["TP"] + by_det[d]["FP"] + by_det[d]["UNC"]
                           for d in gate_dets)
    s2_prec = s2_tp / max(1, s2_n) * 100
    s2_lcb = wilson_lcb(s2_tp, s2_n) * 100
    s2_lcb_bonf = bonferroni_lcb(s2_tp, s2_n) * 100

    # Volume retention
    demoted_findings = sum(by_det[d]["TP"] + by_det[d]["FP"] + by_det[d]["UNC"]
                           for d in DEMOTION_CANDIDATES)

    # Sort all detectors by precision for the per-detector table
    rows_out = []
    for d, c in by_det.items():
        n = c["TP"] + c["FP"]
        p = c["TP"] / max(1, n) * 100
        lcb = wilson_lcb(c["TP"], n) * 100
        rows_out.append({
            "detector": d, "TP": c["TP"], "FP": c["FP"], "UNC": c["UNC"],
            "n": n, "precision": p, "wilson_lcb": lcb,
            "tier_after": "obs" if d in DEMOTION_CANDIDATES else "gate",
        })
    rows_out.sort(key=lambda r: -r["precision"])

    # Print
    print()
    print("=" * 90)
    print("Phase A.5 — Severity-demote null hypothesis")
    print("=" * 90)
    print()
    print("Per-detector breakdown (sorted by precision):")
    print(f"  {'Detector':35s} {'TP':>5s} {'FP':>5s} {'n':>5s} "
          f"{'Prec':>6s} {'W-LCB':>6s} {'Tier→':>6s}")
    print("  " + "-" * 80)
    for r in rows_out:
        print(f"  {r['detector']:35s} {r['TP']:>5d} {r['FP']:>5d} "
              f"{r['n']:>5d} {r['precision']:>5.1f}% {r['wilson_lcb']:>5.1f}% "
              f"{r['tier_after']:>6s}")
    print()

    print("Scenario comparison:")
    print(f"  {'Scenario':40s} {'Findings':>10s} {'TP':>5s} {'FP':>5s} "
          f"{'Prec':>6s} {'W-LCB':>6s} {'Bonf':>6s}")
    print("  " + "-" * 85)
    print(f"  {'S1: all 17 gate-tier (current)':40s} {s1_findings:>10d} "
          f"{s1_tp:>5d} {s1_fp:>5d} {s1_prec:>5.1f}% "
          f"{s1_lcb:>5.1f}% {s1_lcb_bonf:>5.1f}%")
    print(f"  {'S2: 7 demoted to obs (proposed)':40s} {s2_findings_gate:>10d} "
          f"{s2_tp:>5d} {s2_fp:>5d} {s2_prec:>5.1f}% "
          f"{s2_lcb:>5.1f}% {s2_lcb_bonf:>5.1f}%")
    print()

    delta_prec = s2_prec - s1_prec
    vol_kept_pct = s2_findings_gate / max(1, s1_findings) * 100
    print(f"Gate-tier precision lift: {delta_prec:+.1f}pp "
          f"({s1_prec:.1f}% -> {s2_prec:.1f}%)")
    print(f"Wilson LCB lift: {s2_lcb - s1_lcb:+.1f}pp "
          f"({s1_lcb:.1f}% -> {s2_lcb:.1f}%)")
    print(f"Findings volume in gate panel: {vol_kept_pct:.1f}% of pre-demotion "
          f"({s2_findings_gate} of {s1_findings})")
    print(f"Findings moved to observability: {demoted_findings}")
    print()

    # Decision
    print("Decision criteria:")
    print(f"  Gate-tier Wilson LCB >= 85%?    "
          f"{'YES' if s2_lcb >= 85 else 'NO'} ({s2_lcb:.1f}%)")
    print(f"  Bonferroni-adjusted LCB >= 70%? "
          f"{'YES' if s2_lcb_bonf >= 70 else 'NO'} ({s2_lcb_bonf:.1f}%)")
    print()

    if s2_lcb >= 85:
        print("=> Demotion-only baseline CLEARS gate-tier threshold.")
        print("   A1-A5 infrastructure may be unnecessary for cycle 1.")
        print("   The 7 demoted detectors get redesigned for promotion later.")
    elif s2_lcb >= 70:
        print("=> Demotion-only baseline is BORDERLINE.")
        print("   Phase A.1-A.4 experiments still need to run to determine "
              "what additional infrastructure earns its cost.")
    else:
        print("=> Demotion-only baseline does NOT clear gate threshold.")
        print("   Phase A.1-A.4 experiments are required.")
    print()

    # Save full results
    out = {
        "scenario_1_all_gate": {
            "n": s1_n, "findings": s1_findings,
            "TP": s1_tp, "FP": s1_fp,
            "precision_pct": s1_prec,
            "wilson_lcb_pct": s1_lcb,
            "bonferroni_lcb_pct": s1_lcb_bonf,
        },
        "scenario_2_demoted_obs": {
            "n": s2_n, "findings_gate": s2_findings_gate,
            "TP": s2_tp, "FP": s2_fp,
            "precision_pct": s2_prec,
            "wilson_lcb_pct": s2_lcb,
            "bonferroni_lcb_pct": s2_lcb_bonf,
            "demotion_candidates": sorted(DEMOTION_CANDIDATES),
            "findings_moved_to_obs": demoted_findings,
            "volume_kept_pct": vol_kept_pct,
        },
        "per_detector": rows_out,
        "delta": {
            "precision_pp": delta_prec,
            "wilson_lcb_pp": s2_lcb - s1_lcb,
        },
    }
    out_path = Path("tier-4/phase-a5-results.json")
    with out_path.open("w") as f:
        json.dump(out, f, indent=2)
    print(f"Full results written to {out_path}")


if __name__ == "__main__":
    main()
