#!/usr/bin/env python3
"""
Phase B.3 extension — framing test on mockHeavyTest + snapshotHeavyTest.

B.3 proved weakAssertion verdicts flip 55% on strict-vs-pragmatic framing.
Both mockHeavyTest (88.8%) and snapshotHeavyTest (91.7%) are density/ratio
detectors with the same risk: their verdict depends on whether Claude
treats the threshold as a hard rule or a soft signal.

If either detector's flip rate is >15%, gate-tier ship is foreclosed by
the same concept-integrity concern that killed weakAssertion's promotion.

Sample: 12 rows from each detector.
"""

from __future__ import annotations
import json
import random
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path


SEED = 71
N_PER_DETECTOR = 12
TIMEOUT = 120

DETECTORS = ["mockHeavyTest", "snapshotHeavyTest"]


STRICT_TPL = {
    "mockHeavyTest": """You are a STRICT senior QA engineer. Mock-heavy tests don't verify real behavior — they verify what the developer thinks the dependencies do. Heavy use of mocks (especially when mocks outnumber real assertions on real behavior) indicates over-isolation and false confidence.

Detector: {rule_id}
File: {file}
Symbol: {symbol}
Evidence: {evidence}
Title: {title}

Is this finding a TRUE POSITIVE or FALSE POSITIVE?
Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}""",

    "snapshotHeavyTest": """You are a STRICT senior QA engineer. Snapshot-dominated tests obscure the behavior being verified. A test where the snapshot IS the assertion shifts verification cost to the reviewer who must scan opaque output. Inline assertions targeting specific values are preferred.

Detector: {rule_id}
File: {file}
Symbol: {symbol}
Evidence: {evidence}
Title: {title}

Is this finding a TRUE POSITIVE or FALSE POSITIVE?
Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}""",
}

PRAGMATIC_TPL = {
    "mockHeavyTest": """You are a PRAGMATIC senior engineer. Mocks are a legitimate design tool for isolating units under test. A test with many mocks can still verify meaningful behavior if the assertions check the right things. Module-level mocks (vi.mock, jest.mock) for external dependencies are normal in modern test code.

Detector: {rule_id}
File: {file}
Symbol: {symbol}
Evidence: {evidence}
Title: {title}

Is this finding a TRUE POSITIVE or FALSE POSITIVE?
Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}""",

    "snapshotHeavyTest": """You are a PRAGMATIC senior engineer. Snapshot tests are appropriate for complex structured output (HTML, JSON, AST) where inline assertions would be unwieldy. Don't penalize snapshot use unless the snapshot is the entire verification and the underlying contract is unclear.

Detector: {rule_id}
File: {file}
Symbol: {symbol}
Evidence: {evidence}
Title: {title}

Is this finding a TRUE POSITIVE or FALSE POSITIVE?
Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}""",
}


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
        return {"verdict": "CLI_ERROR", "reason": f"exit {result.returncode}"}
    raw = result.stdout.strip()
    start = raw.find("{")
    end = raw.rfind("}")
    if start < 0 or end < 0:
        return {"verdict": "PARSE_ERROR", "reason": raw[:100]}
    try:
        return json.loads(raw[start:end + 1])
    except json.JSONDecodeError:
        return {"verdict": "PARSE_ERROR", "reason": raw[:100]}


def run_for_detector(rows: list[dict], detector: str,
                     rng: random.Random) -> dict:
    pool = [r for r in rows if r.get("rule_id") == detector
            and (r.get("_verdict") or {}).get("verdict") in ("TP", "FP")]
    if len(pool) >= N_PER_DETECTOR:
        sample = rng.sample(pool, N_PER_DETECTOR)
    else:
        sample = pool
    sys.stderr.write(f"[B.3-ext] {detector}: sampling {len(sample)} rows "
                     f"from pool of {len(pool)}\n")

    out_rows = []
    start = time.time()
    for i, r in enumerate(sample, 1):
        kwargs = dict(
            rule_id=r.get("rule_id", ""),
            file=r.get("file", ""),
            symbol=r.get("symbol", ""),
            evidence=(r.get("evidence", "") or "")[:500],
            title=(r.get("title", "") or "")[:200],
        )
        sv = call_claude(STRICT_TPL[detector].format(**kwargs))
        pv = call_claude(PRAGMATIC_TPL[detector].format(**kwargs))
        orig = (r.get("_verdict") or {}).get("verdict", "?")
        out_rows.append({
            "rule_id": detector,
            "file": r.get("file"),
            "symbol": r.get("symbol"),
            "original": orig,
            "strict": sv.get("verdict", "?"),
            "pragmatic": pv.get("verdict", "?"),
            "strict_reason": sv.get("reason", "")[:160],
            "pragmatic_reason": pv.get("reason", "")[:160],
            "agree": sv.get("verdict") == pv.get("verdict"),
        })
        if i % 3 == 0:
            elapsed = time.time() - start
            sys.stderr.write(f"[B.3-ext] {detector} {i}/{len(sample)} "
                             f"@ {2 * i / elapsed:.1f} calls/min\n")

    n = len(out_rows)
    sp_agree = sum(1 for r in out_rows if r["agree"]) / n * 100
    flip_rate = 100 - sp_agree
    pairs = Counter((r["strict"], r["pragmatic"]) for r in out_rows)

    return {
        "detector": detector,
        "n": n,
        "strict_pragmatic_agreement_pct": sp_agree,
        "flip_rate_pct": flip_rate,
        "transitions": {f"{s}->{p}": c for (s, p), c in pairs.items()},
        "rows": out_rows,
    }


def main():
    rows = load_merged()
    rng = random.Random(SEED)

    results = {}
    for det in DETECTORS:
        results[det] = run_for_detector(rows, det, rng)

    print()
    print("=" * 80)
    print("Phase B.3 extension — framing test on density/ratio detectors")
    print("=" * 80)
    for det, r in results.items():
        print()
        print(f"### {det} (n={r['n']})")
        print(f"  Strict <-> Pragmatic agreement: {r['strict_pragmatic_agreement_pct']:.1f}%")
        print(f"  Verdict flip rate: {r['flip_rate_pct']:.1f}%")
        print(f"  Transitions:")
        for trans, c in sorted(r["transitions"].items(), key=lambda kv: -kv[1]):
            print(f"    {trans}: {c}")
        if r["flip_rate_pct"] > 15:
            print(f"  DECISIVE: {det} is style-dependent. Gate-tier foreclosed.")
        elif r["flip_rate_pct"] > 5:
            print(f"  BORDERLINE: {det} has stylistic sensitivity.")
        else:
            print(f"  CLEAN: {det} is structurally stable.")

    out = Path("tier-4/phase-b3-ext-results.json")
    with out.open("w") as f:
        json.dump(results, f, indent=2)
    print()
    print(f"Results: {out}")


if __name__ == "__main__":
    main()
