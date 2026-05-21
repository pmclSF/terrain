#!/usr/bin/env python3
"""
OpenAI cross-rate against the v2 Claude validation baseline.

For each row in tier-4/detector-validation-v2-combined-good.jsonl,
ask OpenAI the same TP/FP question Claude already answered. Compute
Cohen's kappa per detector. Detectors with kappa < 0.6 are
deprioritized for cycle-2 (per cycle-2-master-plan.md, "Honest
gate-tier projection").

The harness handles batching, resume-from, and per-detector kappa.
The caller drives the API spend (their key, their account).

Usage:
    OPENAI_API_KEY=sk-... python3 scripts/cross_rate_openai.py \\
        --in tier-4/detector-validation-v2-combined-good.jsonl \\
        --out tier-4/detector-validation-v2-openai.jsonl \\
        --model gpt-4-turbo \\
        --batch-size 20 \\
        --max-rows 0    # 0 = all 3,208 rows

Resume-from: if --out exists, the harness reads it to identify
already-rated rows by (repo, rule_id, file, symbol) and skips them.

Output: one row per rated finding with the same shape as the input
plus `_openai_verdict: {"verdict": "TP"|"FP", "reason": "..."}`.

Cost estimate: ~3,208 rows × ~600 input tokens × $10/1M tokens = ~$20
for input, plus ~3,208 × 80 output × $30/1M = ~$8. Realistic spend
$30-50 depending on retry/batching overhead.
"""
import argparse
import json
import os
import sys
import time
from collections import Counter
from pathlib import Path


PROMPT_TEMPLATE = """\
You are validating findings emitted by Terrain, a static AI/ML code-quality
analyzer. For each finding, judge whether the detector correctly identified
a real instance (TP) or fired on something unrelated (FP).

DETECTOR RULE: {rule_id}
SEVERITY:      {severity}
TITLE:         {title}
DESCRIPTION:   {description}
EVIDENCE:      {evidence}
FILE:          {file}
SYMBOL:        {symbol}

FILE EXCERPT:
{excerpt}

Output ONLY a JSON object on a single line:
{{"verdict": "TP", "reason": "<one-sentence rationale>"}}
OR
{{"verdict": "FP", "reason": "<one-sentence rationale>"}}

Do not output anything else. No markdown, no preamble.
"""


def load_rows(path):
    rows = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError as e:
                print(f"skip malformed row: {e}", file=sys.stderr)
    return rows


def load_already_rated(path):
    seen = set()
    if not os.path.exists(path):
        return seen
    with open(path) as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            key = (r.get("repo", ""), r.get("rule_id", ""),
                   r.get("file", ""), r.get("symbol", ""))
            seen.add(key)
    return seen


def key_of(row):
    return (row.get("repo", ""), row.get("rule_id", ""),
            row.get("file", ""), row.get("symbol", ""))


def call_openai(row, model):
    """Call OpenAI chat completions. Returns {"verdict": "...", "reason": "..."}.
    Imports openai lazily so the script loads even without the dep installed.
    """
    try:
        from openai import OpenAI
    except ImportError:
        print("error: pip install openai", file=sys.stderr)
        sys.exit(2)

    client = OpenAI(api_key=os.environ.get("OPENAI_API_KEY"))
    prompt = PROMPT_TEMPLATE.format(
        rule_id=row.get("rule_id", ""),
        severity=row.get("severity", ""),
        title=row.get("title", "")[:200],
        description=row.get("description", "")[:400],
        evidence=row.get("evidence", "")[:400],
        file=row.get("file", ""),
        symbol=row.get("symbol", ""),
        excerpt=row.get("_file_excerpt", "")[:2000],
    )

    for attempt in range(3):
        try:
            resp = client.chat.completions.create(
                model=model,
                messages=[{"role": "user", "content": prompt}],
                temperature=0.0,
                max_tokens=200,
            )
            text = resp.choices[0].message.content.strip()
            # Strip code fences if any.
            if text.startswith("```"):
                text = text.split("\n", 1)[1] if "\n" in text else text
                if text.endswith("```"):
                    text = text.rsplit("```", 1)[0]
                text = text.strip()
            return json.loads(text)
        except json.JSONDecodeError:
            return {"verdict": "UNK", "reason": f"non-JSON: {text[:120]}"}
        except Exception as e:
            if attempt == 2:
                return {"verdict": "UNK", "reason": f"openai error: {e}"}
            time.sleep(2 ** attempt)


def cohens_kappa(claude_verdicts, openai_verdicts):
    """Compute Cohen's kappa between two paired lists of TP/FP labels."""
    if len(claude_verdicts) != len(openai_verdicts) or not claude_verdicts:
        return 0.0
    n = len(claude_verdicts)
    agree = sum(1 for c, o in zip(claude_verdicts, openai_verdicts) if c == o)
    po = agree / n
    c1 = Counter(claude_verdicts)
    c2 = Counter(openai_verdicts)
    pe = sum((c1[k] / n) * (c2[k] / n) for k in set(list(c1) + list(c2)))
    if pe == 1.0:
        return 1.0
    return (po - pe) / (1 - pe)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--in", dest="inp",
                    default="tier-4/detector-validation-v2-combined-good.jsonl")
    ap.add_argument("--out",
                    default="tier-4/detector-validation-v2-openai.jsonl")
    ap.add_argument("--model", default="gpt-4-turbo")
    ap.add_argument("--max-rows", type=int, default=0,
                    help="0 = all rows; >0 = first N rows after dedup")
    ap.add_argument("--rules", default="",
                    help="comma-separated rule_id allowlist; empty = all")
    args = ap.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        print("error: OPENAI_API_KEY not set", file=sys.stderr)
        sys.exit(2)

    rows = load_rows(args.inp)
    rule_filter = set(filter(None, [r.strip() for r in args.rules.split(",")]))
    if rule_filter:
        rows = [r for r in rows if r.get("rule_id") in rule_filter]

    seen = load_already_rated(args.out)
    todo = [r for r in rows if key_of(r) not in seen]
    if args.max_rows > 0:
        todo = todo[:args.max_rows]

    print(f"rows total: {len(rows)}")
    print(f"already rated: {len(seen)}")
    print(f"to rate this run: {len(todo)}")

    Path(os.path.dirname(args.out) or ".").mkdir(parents=True, exist_ok=True)

    with open(args.out, "a") as out:
        for i, row in enumerate(todo, 1):
            verdict = call_openai(row, args.model)
            row["_openai_verdict"] = verdict
            out.write(json.dumps(row) + "\n")
            out.flush()
            if i % 20 == 0:
                print(f"  rated {i}/{len(todo)}")

    # Kappa per detector. Load both Claude (from --in) and OpenAI
    # (from --out) verdicts and pair them on key.
    claude_by_key = {}
    for r in rows:
        v = (r.get("_verdict") or {}).get("verdict")
        if v in ("TP", "FP"):
            claude_by_key[key_of(r)] = v

    openai_by_key = {}
    with open(args.out) as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            v = (r.get("_openai_verdict") or {}).get("verdict")
            if v in ("TP", "FP"):
                openai_by_key[key_of(r)] = v

    print("\nPer-detector Cohen's kappa:")
    by_detector = {}
    for key, claude_v in claude_by_key.items():
        openai_v = openai_by_key.get(key)
        if openai_v is None:
            continue
        det = key[1]
        by_detector.setdefault(det, ([], []))
        by_detector[det][0].append(claude_v)
        by_detector[det][1].append(openai_v)

    print(f"{'detector':30s} {'n':>4s} {'kappa':>7s} {'verdict':>20s}")
    for det in sorted(by_detector):
        c_list, o_list = by_detector[det]
        k = cohens_kappa(c_list, o_list)
        verdict = "OK" if k >= 0.6 else "LOW — deprioritize"
        print(f"{det:30s} {len(c_list):4d} {k:7.3f}  {verdict}")


if __name__ == "__main__":
    main()
