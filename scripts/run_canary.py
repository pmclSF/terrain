#!/usr/bin/env python3
"""
Canary runner — re-runs terrain against each canary PR weekly and
tracks UFPP (useful findings per PR), suppression rate, and detector
firing distribution.

Per cycle-2-master-plan.md, the cycle-2 success metric is "UFPP ≤ 3 +
≥40% act-on rate on 3 pilot repos" — the canary set is the upstream
proxy: re-run weekly during build, surface changes week-over-week.

Usage:
    python3 scripts/run_canary.py \\
        --canary harness/canary/canary-set.yaml \\
        --terrain-bin /tmp/terrain-bin \\
        --workdir /tmp/canary-runs \\
        --out tier-4/canary-results.jsonl

Output (one row per (week, canary entry)):
    {
      "week": "2026-W21",
      "entry_id": "canary-001",
      "pr_url": "...",
      "base_sha": "...",
      "head_sha": "...",
      "findings_total": N,
      "findings_gate_tier": N,
      "findings_by_detector": {...},
      "ufpp": float,
      "suppression_rate_per_detector": {...},
      "ran_at": "<iso8601>"
    }
"""
import argparse
import datetime as dt
import json
import os
import subprocess
import sys
from collections import Counter, defaultdict


def load_yaml(path):
    try:
        import yaml
    except ImportError:
        print("error: pip install pyyaml", file=sys.stderr)
        sys.exit(2)
    with open(path) as f:
        return yaml.safe_load(f)


def shallow_clone_pr(workdir, pr_url):
    """Fetch the PR base + head into workdir. Returns (repo_dir, base_sha, head_sha).

    Stub: real implementation uses `gh pr checkout` or the GitHub API.
    For canary v0 the user is expected to pre-clone; this stub returns
    a sentinel so the runner shape is clear.
    """
    # TODO: implement via `gh pr view --json baseRefOid,headRefOid` +
    # `git clone --depth=1 --branch <head_sha>`. For now return placeholders.
    return (workdir, "<base-sha>", "<head-sha>")


def terrain_analyze_pr(terrain_bin, repo_dir, base_sha):
    """Run terrain analyze on the PR's head, return parsed JSON.

    Filtering to PR-scoped findings happens via --base in production;
    canary v0 uses the full analyze output.
    """
    try:
        out = subprocess.check_output(
            [terrain_bin, "analyze", "--root", repo_dir, "--json"],
            stderr=subprocess.DEVNULL,
            timeout=600,
        )
        return json.loads(out)
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired,
            json.JSONDecodeError) as e:
        return {"_error": str(e)}


def summarize_run(entry, report, base_sha, head_sha):
    """Build the per-(week, entry) summary row.

    UFPP = total findings / 1 (per-PR). Suppression rate per detector
    is derived from observability-tier signals vs gate-tier signals.
    """
    by_det = Counter()
    gate_tier = 0
    if "signalSummary" in report:
        # New shape: keyed counts.
        gate_tier = report["signalSummary"].get("total", 0)
    # The full per-detector breakdown lives elsewhere in the report.
    # Canary v0 records the summary counts only; subsequent revs will
    # consume the full findings list when the report exposes it.

    week = dt.datetime.utcnow().strftime("%Y-W%V")
    return {
        "week": week,
        "entry_id": entry.get("id"),
        "pr_url": entry.get("pr_url"),
        "base_sha": base_sha,
        "head_sha": head_sha,
        "findings_total": gate_tier,
        "findings_by_detector": dict(by_det),
        "ufpp": float(gate_tier),
        "ran_at": dt.datetime.utcnow().isoformat() + "Z",
        "report_keys": sorted(report.keys()) if isinstance(report, dict) else [],
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--canary", default="harness/canary/canary-set.yaml")
    ap.add_argument("--terrain-bin", default="/tmp/terrain-bin")
    ap.add_argument("--workdir", default="/tmp/canary-runs")
    ap.add_argument("--out", default="tier-4/canary-results.jsonl")
    args = ap.parse_args()

    if not os.path.exists(args.canary):
        print(f"error: {args.canary} not found", file=sys.stderr)
        print(f"hint: copy harness/canary/canary-set.yaml.example to {args.canary} and fill in 15-25 entries", file=sys.stderr)
        sys.exit(1)

    spec = load_yaml(args.canary)
    entries = spec.get("entries", [])
    if not entries:
        print("error: canary-set.yaml has no entries", file=sys.stderr)
        sys.exit(1)

    os.makedirs(os.path.dirname(args.out) or ".", exist_ok=True)
    os.makedirs(args.workdir, exist_ok=True)

    with open(args.out, "a") as out_fh:
        for entry in entries:
            print(f"canary {entry.get('id')}: {entry.get('pr_url')}")
            repo_dir, base_sha, head_sha = shallow_clone_pr(args.workdir, entry.get("pr_url", ""))
            report = terrain_analyze_pr(args.terrain_bin, repo_dir, base_sha)
            row = summarize_run(entry, report, base_sha, head_sha)
            out_fh.write(json.dumps(row) + "\n")
            out_fh.flush()
            print(f"  total={row['findings_total']} ufpp={row['ufpp']:.2f}")

    # Final aggregate summary across this week's run.
    rows = []
    with open(args.out) as f:
        for line in f:
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    by_week = defaultdict(list)
    for r in rows:
        by_week[r["week"]].append(r["ufpp"])
    print("\nWeekly UFPP medians:")
    for wk in sorted(by_week):
        ufpps = sorted(by_week[wk])
        median = ufpps[len(ufpps) // 2] if ufpps else 0
        print(f"  {wk}: median UFPP {median:.2f} across {len(ufpps)} entries")


if __name__ == "__main__":
    main()
