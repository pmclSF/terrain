#!/usr/bin/env python3
"""
Phase B.3 — Stylistic-bias test.

weakAssertion FPs frequently cite stylistic judgments ("density 0.5/test is
normal for focused tests" vs "density 0.5/test reflects thin verification").
Question: would the same row get a different verdict if Claude were
framed as a "strict reviewer" vs a "pragmatic engineer"?

If verdicts flip >15% between the two framings, weakAssertion's precision
is partly a style poll and the detector should not be gate-tier.

Sample: 20 weakAssertion borderline rows (density 0.5-1.5 assertions/test or
verdict reason mentions "density").
"""

from __future__ import annotations
import json
import random
import re
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path


SEED = 53
N_SAMPLE = 20
TIMEOUT = 90


STRICT_PROMPT = """You are a STRICT senior QA engineer reviewing code quality. You hold tests to high standards: shallow assertions or low assertion density indicate incomplete verification, even if the test "passes." A test should verify *meaningful* behavior with multiple precise checks.

Detector: {rule_id}
Severity: {severity}
File: {file}
Symbol: {symbol}
Evidence: {evidence}
Title: {title}

Is this finding a TRUE POSITIVE (real quality issue worth fixing) or FALSE POSITIVE (acceptable test, not worth flagging)?

Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}
"""

PRAGMATIC_PROMPT = """You are a PRAGMATIC senior engineer reviewing code quality. You judge tests by whether they catch real bugs, not by style or count of assertions. A focused test with one strong assertion is often better than a sprawling test with many shallow ones. Don't penalize tests for being minimal if they're doing their job.

Detector: {rule_id}
Severity: {severity}
File: {file}
Symbol: {symbol}
Evidence: {evidence}
Title: {title}

Is this finding a TRUE POSITIVE (real quality issue worth fixing) or FALSE POSITIVE (acceptable test, not worth flagging)?

Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}
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
                "reason": f"exit {result.returncode}"}
    raw = result.stdout.strip()
    start = raw.find("{")
    end = raw.rfind("}")
    if start < 0 or end < 0:
        return {"verdict": "PARSE_ERROR", "reason": raw[:100]}
    try:
        return json.loads(raw[start:end + 1])
    except json.JSONDecodeError:
        return {"verdict": "PARSE_ERROR", "reason": raw[:100]}


def main():
    rows = load_merged()
    weak = [r for r in rows if r.get("rule_id") == "weakAssertion"
            and (r.get("_verdict") or {}).get("verdict") in ("TP", "FP")]
    # Borderline: reason mentions density or short
    borderline_re = re.compile(
        r"(density|focused|minimal|adequate|sufficient|sparse|thin|"
        r"borderline|acceptable|reasonable|normal|standard|"
        r"low|weak|shallow|simple)", re.IGNORECASE)
    borderline = [r for r in weak
                  if borderline_re.search(
                      (r.get("_verdict") or {}).get("reason", ""))]
    print(f"weakAssertion total: {len(weak)}; borderline: {len(borderline)}",
          file=sys.stderr)

    rng = random.Random(SEED)
    if len(borderline) >= N_SAMPLE:
        sample = rng.sample(borderline, N_SAMPLE)
    else:
        sample = borderline + rng.sample(
            [r for r in weak if r not in borderline],
            N_SAMPLE - len(borderline))

    out_rows = []
    start = time.time()
    for i, r in enumerate(sample, 1):
        kwargs = dict(
            rule_id=r.get("rule_id", ""),
            severity=r.get("severity", "medium"),
            file=r.get("file", ""),
            symbol=r.get("symbol", ""),
            evidence=(r.get("evidence", "") or "")[:500],
            title=(r.get("title", "") or "")[:200],
        )
        strict_v = call_claude(STRICT_PROMPT.format(**kwargs))
        prag_v = call_claude(PRAGMATIC_PROMPT.format(**kwargs))
        orig = (r.get("_verdict") or {}).get("verdict", "?")
        s = strict_v.get("verdict", "?")
        p = prag_v.get("verdict", "?")
        out_rows.append({
            "rule_id": r.get("rule_id"),
            "file": r.get("file"),
            "symbol": r.get("symbol"),
            "original": orig,
            "strict": s,
            "pragmatic": p,
            "strict_pragmatic_agree": s == p,
            "strict_matches_original": s == orig,
            "pragmatic_matches_original": p == orig,
            "strict_reason": strict_v.get("reason", "")[:160],
            "pragmatic_reason": prag_v.get("reason", "")[:160],
        })
        if i % 5 == 0:
            elapsed = time.time() - start
            sys.stderr.write(f"[B.3] {i}/{len(sample)} "
                             f"@ {2 * i / elapsed:.1f} calls/min\n")

    # Stats
    n = len(out_rows)
    sp_agree = sum(1 for r in out_rows if r["strict_pragmatic_agree"]) / n * 100
    strict_orig = sum(1 for r in out_rows
                      if r["strict_matches_original"]) / n * 100
    prag_orig = sum(1 for r in out_rows
                    if r["pragmatic_matches_original"]) / n * 100
    sp_pairs = Counter((r["strict"], r["pragmatic"]) for r in out_rows)

    print()
    print("=" * 80)
    print("Phase B.3 — weakAssertion stylistic-bias test")
    print("=" * 80)
    print()
    print(f"Rows tested: {n}")
    print(f"Strict-prompt vs Pragmatic-prompt agreement: {sp_agree:.1f}%")
    print(f"Strict-prompt vs original verdict agreement: {strict_orig:.1f}%")
    print(f"Pragmatic-prompt vs original verdict agreement: {prag_orig:.1f}%")
    print()
    print(f"Strict / Pragmatic transitions:")
    for (s, p), c in sp_pairs.most_common():
        print(f"  strict={s:>10s}  pragmatic={p:>10s}: {c:>3d}")
    print()

    flip_rate = 100 - sp_agree
    print(f"Verdict flip rate between framings: {flip_rate:.1f}%")
    if flip_rate > 15:
        print("DECISIVE: weakAssertion's verdict is style-dependent.")
        print("Cannot be gate-tier — the precision number is a style poll.")
        print("Recommend permanent observability tier.")
    elif flip_rate > 5:
        print("BORDERLINE: weakAssertion has stylistic sensitivity.")
        print("Acceptable for observability tier; gate-tier needs further work.")
    else:
        print("CLEAN: framing doesn't change verdicts — weakAssertion is "
              "a structural signal, not a style judgment.")
    print()

    # Show flipped rows
    flipped = [r for r in out_rows if not r["strict_pragmatic_agree"]]
    if flipped:
        print("Examples of strict/pragmatic disagreement:")
        for r in flipped[:5]:
            print(f"  {r['file']:60s} sym={r['symbol']}")
            print(f"    strict={r['strict']:>10s}: {r['strict_reason']}")
            print(f"    pragm={r['pragmatic']:>10s}: {r['pragmatic_reason']}")
            print()

    out = Path("tier-4/phase-b3-results.json")
    with out.open("w") as f:
        json.dump({
            "flip_rate_pct": flip_rate,
            "strict_pragmatic_agreement_pct": sp_agree,
            "strict_vs_original_pct": strict_orig,
            "pragmatic_vs_original_pct": prag_orig,
            "rows": out_rows,
        }, f, indent=2)
    print(f"Results: {out}")


if __name__ == "__main__":
    main()
