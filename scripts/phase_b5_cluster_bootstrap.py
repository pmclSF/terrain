#!/usr/bin/env python3
"""
Phase B.5 — Cluster-bootstrap precision CIs by unique repo.

The Wilson CIs in Phase A.5 assume row-independence. But multiple findings
from the same repo share priors, project conventions, and code style — they
are NOT independent observations. This script recomputes per-detector
precision CIs by bootstrap-resampling at the REPO level, giving honest
"effective-N" confidence bounds.

A common pattern: a detector shows precision=89% on n=250 rows but those
rows come from 37 unique repos with one repo contributing 67 rows. The
row-level CI is ±4pp; the repo-clustered CI may be ±10pp or wider.
"""

from __future__ import annotations
import json
import random
import sys
from collections import defaultdict
from pathlib import Path


N_BOOTSTRAP = 2000
SEED = 43


def load_merged() -> list[dict]:
    rows = []
    for path in ["tier-4/detector-validation.jsonl",
                 "tier-4/detector-validation-n200.jsonl"]:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if line:
                    try:
                        rows.append(json.loads(line))
                    except json.JSONDecodeError:
                        continue
    seen = set()
    dedup = []
    for r in rows:
        k = (r.get("repo", ""), r.get("rule_id", ""),
             r.get("file", ""), r.get("symbol", ""))
        if k in seen:
            continue
        seen.add(k)
        dedup.append(r)
    return dedup


def verdict_of(r: dict) -> str:
    return (r.get("_verdict") or {}).get("verdict", "?")


def precision_of(rows: list[dict]) -> float | None:
    tp = sum(1 for r in rows if verdict_of(r) == "TP")
    fp = sum(1 for r in rows if verdict_of(r) == "FP")
    if tp + fp == 0:
        return None
    return tp / (tp + fp)


def main():
    rows = load_merged()
    print(f"loaded {len(rows)} merged rows", file=sys.stderr)

    # Per-detector breakdown
    by_det = defaultdict(list)
    for r in rows:
        by_det[r.get("rule_id", "?")].append(r)

    rng = random.Random(SEED)

    summary = []
    for det, det_rows in sorted(by_det.items()):
        tp = sum(1 for r in det_rows if verdict_of(r) == "TP")
        fp = sum(1 for r in det_rows if verdict_of(r) == "FP")
        if tp + fp < 5:
            continue
        point = tp / (tp + fp)

        # Group by repo
        by_repo = defaultdict(list)
        for r in det_rows:
            by_repo[r.get("repo", "")].append(r)
        unique_repos = list(by_repo.keys())

        # Concentration metric: rows from top repo / total
        top_repo_count = max(len(rs) for rs in by_repo.values())
        top_repo_share = top_repo_count / (tp + fp)

        # Cluster bootstrap at repo level
        precs = []
        for _ in range(N_BOOTSTRAP):
            sampled_repos = [rng.choice(unique_repos)
                             for _ in range(len(unique_repos))]
            sampled_rows = []
            for repo in sampled_repos:
                sampled_rows.extend(by_repo[repo])
            p = precision_of(sampled_rows)
            if p is not None:
                precs.append(p)
        precs.sort()
        if precs:
            lci_cluster = precs[int(len(precs) * 0.025)]
            uci_cluster = precs[int(len(precs) * 0.975)]
        else:
            lci_cluster = uci_cluster = 0

        # For comparison: naive Wilson row-level (matches Phase A.5)
        # Approximation: 1.96 * sqrt(p(1-p)/n)
        n = tp + fp
        from math import sqrt
        if n > 0:
            naive_se = sqrt(point * (1 - point) / n)
            wilson_lci = max(0, point - 1.96 * naive_se)
            wilson_uci = min(1, point + 1.96 * naive_se)
        else:
            wilson_lci = wilson_uci = 0

        summary.append({
            "detector": det,
            "n_rows": n,
            "n_repos": len(unique_repos),
            "top_repo_share_pct": top_repo_share * 100,
            "precision_pct": point * 100,
            "wilson_row_lci_pct": wilson_lci * 100,
            "wilson_row_uci_pct": wilson_uci * 100,
            "cluster_lci_pct": lci_cluster * 100,
            "cluster_uci_pct": uci_cluster * 100,
            "row_width_pp": (wilson_uci - wilson_lci) * 100,
            "cluster_width_pp": (uci_cluster - lci_cluster) * 100,
            "ci_widening_x": ((uci_cluster - lci_cluster) /
                              max(0.001, wilson_uci - wilson_lci)),
        })

    summary.sort(key=lambda r: -r["precision_pct"])

    print()
    print("=" * 110)
    print("Phase B.5 — Cluster-bootstrap precision CIs (by unique repo)")
    print("=" * 110)
    print()
    print(f"{'Detector':30s} {'n':>4s} {'#repos':>6s} {'top%':>5s} "
          f"{'Prec':>5s}  {'Wilson row CI':>18s}  {'Cluster CI':>18s}  "
          f"{'Wide?':>6s}")
    print("-" * 110)
    for s in summary:
        wilson = f"[{s['wilson_row_lci_pct']:5.1f}, {s['wilson_row_uci_pct']:5.1f}]"
        cluster = f"[{s['cluster_lci_pct']:5.1f}, {s['cluster_uci_pct']:5.1f}]"
        widening = f"{s['ci_widening_x']:.1f}x"
        print(f"{s['detector']:30s} {s['n_rows']:>4d} "
              f"{s['n_repos']:>6d} {s['top_repo_share_pct']:>4.0f}% "
              f"{s['precision_pct']:>4.1f}%  {wilson:>18s}  {cluster:>18s}  "
              f"{widening:>6s}")
    print()

    # Highlight detectors where row-CI is misleading
    misleading = [s for s in summary if s["ci_widening_x"] > 1.8]
    if misleading:
        print("Detectors where cluster CI is materially wider "
              "(row CI was misleading):")
        for s in misleading:
            print(f"  {s['detector']:30s}  cluster CI is "
                  f"{s['ci_widening_x']:.1f}x wider than row CI  "
                  f"(top repo = {s['top_repo_share_pct']:.0f}% of rows)")
        print()

    # Save
    out = Path("tier-4/phase-b5-results.json")
    with out.open("w") as f:
        json.dump(summary, f, indent=2)
    print(f"Results JSON: {out}")


if __name__ == "__main__":
    main()
