#!/usr/bin/env python3
"""
Phase B.1 — Claude self-consistency.

Re-rate 50 stratified rows via Claude CLI a second time. Compute Cohen's
kappa between original verdict and new verdict.

Note: original rating included a _file_excerpt that isn't saved with the
row. This re-rate uses metadata + evidence ONLY (no file content). So
this measures "is Claude consistent on the row's textual metadata?" which
is a weaker test than strict same-input kappa, but tests an important
property: if Claude is unstable on this prompt shape, the entire verdict
pipeline has a noise floor that wasn't accounted for.

Target: Cohen's kappa >= 0.75. Below 0.6, the entire corpus is suspect.
"""

from __future__ import annotations
import json
import random
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path


SEED = 47
PER_DETECTOR = 3   # 3 rows per detector × 17 detectors = ~51 rows
TIMEOUT = 90


PROMPT_TEMPLATE = """You are a senior software engineer reviewing a static-analysis finding produced by a code-quality tool.

Detector: {rule_id}
Severity: {severity}
Title: {title}
Description: {description}
Repository: {repo}
File: {file}
Symbol: {symbol}
Evidence: {evidence}

Is this finding a TRUE POSITIVE (real problem the team should fix) or a FALSE POSITIVE (noise, not worth flagging)?

Consider:
- Does the detector's claim hold given the file path, symbol, and evidence?
- Would a competent engineer at this codebase agree this is worth attention?
- Are there obvious context-aware reasons this finding shouldn't fire?

Respond in this JSON shape only (no other text):
{{"verdict": "TP", "reason": "<one line>"}}
or
{{"verdict": "FP", "reason": "<one line>"}}
or
{{"verdict": "UNCERTAIN", "reason": "<one line>"}}
"""


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


def stratified_sample(rows: list[dict], per_det: int,
                      rng: random.Random) -> list[dict]:
    by_det = defaultdict(list)
    for r in rows:
        d = r.get("rule_id", "?")
        v = (r.get("_verdict") or {}).get("verdict", "?")
        if v in ("TP", "FP"):
            by_det[d].append(r)
    sample = []
    for d, rs in by_det.items():
        if len(rs) >= per_det:
            sample.extend(rng.sample(rs, per_det))
        else:
            sample.extend(rs)
    return sample


def call_claude(prompt: str) -> dict:
    prompt = prompt.replace("\x00", "")
    try:
        result = subprocess.run(
            ["claude", "-p", prompt],
            capture_output=True, text=True, timeout=TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        return {"verdict": "TIMEOUT", "reason": "claude timeout"}
    if result.returncode != 0:
        return {"verdict": "CLI_ERROR",
                "reason": f"exit {result.returncode}: {result.stderr[:200]}"}
    raw = result.stdout.strip()
    start = raw.find("{")
    end = raw.rfind("}")
    if start < 0 or end < 0:
        return {"verdict": "PARSE_ERROR", "reason": raw[:100]}
    try:
        return json.loads(raw[start:end + 1])
    except json.JSONDecodeError:
        return {"verdict": "PARSE_ERROR", "reason": raw[:100]}


def cohens_kappa(pairs: list[tuple[str, str]]) -> float:
    """Two-rater Cohen's kappa on categorical verdicts."""
    if not pairs:
        return 0.0
    labels = sorted({a for a, b in pairs} | {b for a, b in pairs})
    n = len(pairs)
    p_o = sum(1 for a, b in pairs if a == b) / n
    p_e = 0.0
    for lab in labels:
        p1 = sum(1 for a, _ in pairs if a == lab) / n
        p2 = sum(1 for _, b in pairs if b == lab) / n
        p_e += p1 * p2
    if p_e >= 1.0:
        return 0.0
    return (p_o - p_e) / (1 - p_e)


def main():
    rows = load_merged()
    rng = random.Random(SEED)
    sample = stratified_sample(rows, PER_DETECTOR, rng)
    print(f"sampled {len(sample)} rows across "
          f"{len({r['rule_id'] for r in sample})} detectors", file=sys.stderr)

    start = time.time()
    out_rows = []
    for i, r in enumerate(sample, 1):
        prompt = PROMPT_TEMPLATE.format(
            rule_id=r.get("rule_id", ""),
            severity=r.get("severity", "medium"),
            title=(r.get("title", "") or "")[:200],
            description=(r.get("description", "") or "")[:400],
            repo=r.get("repo", ""),
            file=r.get("file", ""),
            symbol=r.get("symbol", ""),
            evidence=(r.get("evidence", "") or "")[:500],
        )
        new_verdict = call_claude(prompt)
        orig = (r.get("_verdict") or {}).get("verdict", "?")
        new = new_verdict.get("verdict", "?")
        out_rows.append({
            "rule_id": r.get("rule_id"),
            "repo": r.get("repo"),
            "file": r.get("file"),
            "symbol": r.get("symbol"),
            "original_verdict": orig,
            "original_reason": (r.get("_verdict") or {}).get("reason", "")[:200],
            "new_verdict": new,
            "new_reason": new_verdict.get("reason", "")[:200],
            "agree": orig == new,
        })
        if i % 5 == 0:
            elapsed = time.time() - start
            sys.stderr.write(
                f"[B.1] {i}/{len(sample)} @ {i / elapsed:.2f}/s\n"
            )

    # Compute kappa
    pairs = [(r["original_verdict"], r["new_verdict"]) for r in out_rows]
    kappa = cohens_kappa(pairs)
    agree_pct = sum(1 for r in out_rows if r["agree"]) / len(out_rows) * 100

    # Per-detector breakdown
    per_det = defaultdict(lambda: {"total": 0, "agree": 0,
                                    "flip_tp_to_fp": 0, "flip_fp_to_tp": 0,
                                    "to_uncertain": 0})
    for r in out_rows:
        d = r["rule_id"]
        per_det[d]["total"] += 1
        if r["agree"]:
            per_det[d]["agree"] += 1
        elif r["original_verdict"] == "TP" and r["new_verdict"] == "FP":
            per_det[d]["flip_tp_to_fp"] += 1
        elif r["original_verdict"] == "FP" and r["new_verdict"] == "TP":
            per_det[d]["flip_fp_to_tp"] += 1
        elif r["new_verdict"] == "UNCERTAIN":
            per_det[d]["to_uncertain"] += 1

    # Confusion table
    pair_counts = Counter(pairs)

    print()
    print("=" * 90)
    print("Phase B.1 — Claude self-consistency on metadata-only prompts")
    print("=" * 90)
    print()
    print(f"Rows re-rated: {len(out_rows)}")
    print(f"Observed agreement: {agree_pct:.1f}%")
    print(f"Cohen's kappa: {kappa:.3f}")
    print()
    print("Kappa interpretation:")
    print("  >= 0.81  almost perfect")
    print("  0.61-0.80  substantial")
    print("  0.41-0.60  moderate")
    print("  0.21-0.40  fair")
    print("  <  0.20  slight")
    print()

    print("Verdict transition matrix (original -> new):")
    verdicts = sorted({a for a, _ in pairs} | {b for _, b in pairs})
    header = "             " + "  ".join(f"{v:>10s}" for v in verdicts)
    print(header)
    for orig in verdicts:
        row = f"  orig={orig:<5s} "
        for new in verdicts:
            row += f"  {pair_counts[(orig, new)]:>10d}"
        print(row)
    print()

    print("Per-detector agreement (only detectors with disagreement):")
    print(f"  {'Detector':30s} {'n':>3s} {'agree':>6s} "
          f"{'TP→FP':>6s} {'FP→TP':>6s} {'→UNC':>5s}")
    print("  " + "-" * 70)
    for d in sorted(per_det, key=lambda k: per_det[k]["agree"] / max(1, per_det[k]["total"])):
        c = per_det[d]
        if c["agree"] == c["total"]:
            continue  # skip perfect agreement
        agree_p = c["agree"] / c["total"] * 100
        print(f"  {d:30s} {c['total']:>3d} {c['agree']:>5d} "
              f"({agree_p:>3.0f}%) {c['flip_tp_to_fp']:>5d} "
              f"{c['flip_fp_to_tp']:>5d} {c['to_uncertain']:>5d}")
    print()

    # Decision
    print("Decision:")
    if kappa >= 0.75:
        print(f"  Kappa {kappa:.3f} >= 0.75 — Claude is substantially consistent.")
        print("  Existing verdicts are trustworthy enough to plan from.")
    elif kappa >= 0.60:
        print(f"  Kappa {kappa:.3f} in [0.60, 0.75) — borderline.")
        print("  Recommend running B.2 (truncation) before trusting per-row "
              "verdicts for ship/no-ship decisions.")
    else:
        print(f"  Kappa {kappa:.3f} < 0.60 — Claude is NOT consistent.")
        print("  The entire corpus has a noise floor that materially affects "
              "all precision estimates. Consider re-rating with stronger "
              "context (full file content) or seeking a second oracle.")
    print()

    out = Path("tier-4/phase-b1-results.json")
    with out.open("w") as f:
        json.dump({
            "kappa": kappa,
            "agreement_pct": agree_pct,
            "n_rated": len(out_rows),
            "transitions": {f"{a}->{b}": c for (a, b), c in pair_counts.items()},
            "per_detector": dict(per_det),
            "rows": out_rows,
        }, f, indent=2)
    print(f"Full results: {out}")


if __name__ == "__main__":
    main()
