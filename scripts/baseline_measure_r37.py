#!/usr/bin/env python3
"""
R3.7: v2 baseline measurement for the four claim-without-evidence detectors.

Per cycle-2-master-plan.md R3.7:
- aiPromptVersioning (re-validate post-Phase-C)
- aiPromptInjectionRisk
- aiHardcodedAPIKey
- testsOnlyMocks

Cycle-2 deliverable is v2-baseline measurement (n>=150 each), NOT a
lift target. Engineering of SSSG, PFT, and the "narrow to AI-client-
mock" reform is deferred to cycle-3 pending baseline.

This script is a thin wrapper around the existing Claude validator
(scripts/validate_detectors_claude.py). It runs terrain on the
configured corpus, filters per-detector findings to the four
claim-without-evidence detectors, samples up to N per detector, and
rates each with Claude.

Usage:
    python3 scripts/baseline_measure_r37.py \\
        --repo-list tier-4/sample-repos.txt \\
        --terrain-bin /tmp/terrain-bin \\
        --output tier-4/r37-baseline.jsonl \\
        --per-detector 150

After the run completes, summarize per-detector precision via
the same _verdict aggregation pattern as the v2 corpus.
"""
import argparse
import json
import os
import subprocess
import sys
from collections import Counter


R37_DETECTORS = [
    "aiPromptVersioning",
    "aiPromptInjectionRisk",
    "aiHardcodedAPIKey",
    "testsOnlyMocks",
]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo-list", required=True,
                    help="file with one owner__name per line")
    ap.add_argument("--terrain-bin", default="/tmp/terrain-bin")
    ap.add_argument("--workdir", default="/tmp/r37-baseline")
    ap.add_argument("--output", default="tier-4/r37-baseline.jsonl")
    ap.add_argument("--per-detector", type=int, default=150)
    ap.add_argument("--max-repos", type=int, default=500)
    ap.add_argument("--summary-only", action="store_true",
                    help="Skip Claude rating; just print existing _verdict totals")
    args = ap.parse_args()

    if args.summary_only:
        summarize(args.output)
        return

    # Delegate to the existing detector validator. The wrapper passes
    # tp-per-detector=N and the four detector names; validator handles
    # cloning, terrain scan, sampling, and Claude rating.
    cmd = [
        "python3", "scripts/validate_detectors_claude.py",
        "--repo-list", args.repo_list,
        "--terrain-bin", args.terrain_bin,
        "--workdir", args.workdir,
        "--output", args.output,
        "--tp-per-detector", str(args.per_detector),
        "--max-repos", str(args.max_repos),
    ]
    print("run:", " ".join(cmd))
    rc = subprocess.call(cmd)
    if rc != 0:
        print(f"validator exited {rc}", file=sys.stderr)
        sys.exit(rc)

    summarize(args.output)


def summarize(path):
    """Aggregate _verdict counts per detector, R3.8-style."""
    if not os.path.exists(path):
        print(f"no output at {path}; nothing to summarize", file=sys.stderr)
        sys.exit(1)
    by_det = {}
    with open(path) as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            det = r.get("rule_id", "?")
            if det not in R37_DETECTORS:
                continue
            v = (r.get("_verdict") or {}).get("verdict", "?")
            by_det.setdefault(det, Counter())[v] += 1

    print("\nR3.7 baseline measurement (claim-without-evidence detectors):")
    print(f"{'detector':30s} {'n':>4s} {'TP':>4s} {'FP':>4s} {'UNK':>4s} {'prec':>6s}")
    for det in R37_DETECTORS:
        c = by_det.get(det, Counter())
        n = sum(c.values())
        tp = c.get("TP", 0)
        fp = c.get("FP", 0)
        unk = c.get("UNK", 0) + c.get("UNKNOWN", 0)
        prec = tp / (tp + fp) if (tp + fp) else 0
        marker = "" if n >= 150 else f"  (under-sampled: target 150)"
        print(f"{det:30s} {n:4d} {tp:4d} {fp:4d} {unk:4d} {prec*100:5.1f}%{marker}")


if __name__ == "__main__":
    main()
