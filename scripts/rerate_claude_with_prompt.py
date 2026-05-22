#!/usr/bin/env python3
"""
Re-rate rows from a v2-style validation JSONL using Claude CLI with a
custom prompt version from cross_rate_openai.py's PROMPT_VERSIONS table.

This exists because the v3 anti-anchor prompt diverges from the v2
prompt used to produce tier-4/detector-validation-v2-combined-good.jsonl.
Cohen's kappa requires both raters to see the same prompt; this script
produces a v3-Claude-rated JSONL that can then be fed to
cross_rate_openai.py --prompt-version v3-anti-anchor for a paired
comparison.

Schema:
  Input row has `_verdict` from the v2 prompt.
  Output row has:
    `_verdict`         <- the new v3 verdict (so downstream code Just Works)
    `_verdict_v2`      <- the original v2 verdict, preserved
    `_prompt_version`  <- "v3-anti-anchor"
    `_rater`           <- "claude-cli"

Usage:
    python3 scripts/rerate_claude_with_prompt.py \
        --in tier-4/detector-validation-v2-combined-good.jsonl \
        --out tier-4/detector-validation-v3-claude-smoke.jsonl \
        --rules deprecatedTestPattern,aiNonDeterministicEval,aiSafetyEvalMissing,uncoveredAISurface,promptFileMissingEval \
        --max-rows 20 \
        --prompt-version v3-anti-anchor

The same `--rules` and `--max-rows` (with the same stratified-sample seed)
guarantee the SAME 20 rows the OpenAI cross-rate smoke processed —
so you can pair them by dedup key for kappa.

Cost: Claude CLI is on subscription ($0). Time: ~10s/row sequential.
20-row smoke ~3 min. Full 5-detector run (~988 rows) ~3 hours.
"""
import argparse
import json
import os
import re
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path

# Import the prompt table from cross_rate_openai so the two scripts can
# never drift. Same module path, sibling file.
HERE = Path(__file__).parent.resolve()
sys.path.insert(0, str(HERE))
import cross_rate_openai as cr  # noqa: E402


JSON_RE = re.compile(r"\{[^{}]*\"verdict\"[^{}]*\}", re.DOTALL)


def parse_verdict(raw: str) -> dict:
    if not raw:
        return {"verdict": "PARSE_ERROR", "reason": "empty"}
    raw = raw.strip()
    if raw.startswith("```"):
        raw = raw.split("\n", 1)[1] if "\n" in raw else raw
        if raw.endswith("```"):
            raw = raw.rsplit("```", 1)[0]
        raw = raw.strip()
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        pass
    m = JSON_RE.search(raw)
    if m:
        try:
            return json.loads(m.group(0))
        except json.JSONDecodeError:
            pass
    return {"verdict": "PARSE_ERROR", "reason": "extract-fail",
            "raw_head": raw[:200]}


def call_claude(prompt: str, timeout: int) -> dict:
    # subprocess.run rejects strings with embedded null bytes.
    prompt = prompt.replace("\x00", "")
    try:
        proc = subprocess.run(
            ["claude", "-p", prompt],
            capture_output=True, text=True, timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return {"verdict": "TIMEOUT", "reason": f"claude >{timeout}s"}
    except FileNotFoundError:
        sys.exit("ERROR: claude CLI not in PATH")
    if proc.returncode != 0:
        return {"verdict": "CLI_ERROR", "reason": f"exit={proc.returncode}",
                "stderr_head": (proc.stderr or "")[:200]}
    return parse_verdict(proc.stdout)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--in", dest="inp",
                    default="tier-4/detector-validation-v2-combined-good.jsonl")
    ap.add_argument("--out", required=True)
    ap.add_argument("--prompt-version", default="v3-anti-anchor",
                    choices=sorted(cr.PROMPT_VERSIONS.keys()))
    ap.add_argument("--rules", default="",
                    help="comma-separated rule_id allowlist; empty = all")
    ap.add_argument("--max-rows", type=int, default=0,
                    help="0 = all rows after dedup; >0 = stratified sample of N rows")
    ap.add_argument("--timeout", type=int, default=90)
    ap.add_argument("--progress-every", type=int, default=5)
    args = ap.parse_args()

    rows = cr.load_rows(args.inp)
    rule_filter = set(filter(None, [r.strip() for r in args.rules.split(",")]))
    if rule_filter:
        rows = [r for r in rows if r.get("rule_id") in rule_filter]

    # Resume: skip keys already rated by THIS prompt version.
    seen = set()
    if os.path.exists(args.out):
        with open(args.out) as f:
            for line in f:
                try:
                    r = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if r.get("_prompt_version") != args.prompt_version:
                    continue
                if r.get("_rater") != "claude-cli":
                    continue
                v = (r.get("_verdict") or {}).get("verdict")
                if v in ("TP", "FP", "UNCERTAIN"):
                    seen.add(cr.key_of(r))

    todo = [r for r in rows if cr.key_of(r) not in seen]
    # Same seed (42) as cross_rate_openai → same sample selection.
    if args.max_rows > 0 and len(todo) > args.max_rows:
        todo = cr.stratified_sample(todo, args.max_rows,
                                    lambda r: r.get("rule_id", ""), seed=42)

    sys.stderr.write(f"input rows (after rule filter): {len(rows)}\n")
    sys.stderr.write(f"already rated ({args.prompt_version}, claude-cli): {len(seen)}\n")
    sys.stderr.write(f"to rate this run: {len(todo)}\n")
    sys.stderr.write(f"prompt={args.prompt_version} timeout={args.timeout}s\n\n")

    Path(os.path.dirname(args.out) or ".").mkdir(parents=True, exist_ok=True)

    out = open(args.out, "a")
    counts = Counter()
    start = time.time()
    for i, row in enumerate(todo, 1):
        prompt = cr.build_prompt(row, args.prompt_version)
        verdict = call_claude(prompt, args.timeout)
        out_row = dict(row)
        # Preserve the original (v2 prompt) verdict for audit.
        if "_verdict" in row:
            out_row["_verdict_v2"] = row["_verdict"]
        out_row["_verdict"] = verdict
        out_row["_prompt_version"] = args.prompt_version
        out_row["_rater"] = "claude-cli"
        out.write(json.dumps(out_row) + "\n")
        out.flush()

        v = verdict.get("verdict", "?")
        counts[v] += 1
        if i % args.progress_every == 0 or i == len(todo):
            elapsed = time.time() - start
            rate = i / elapsed if elapsed else 0
            sys.stderr.write(
                f"[claude-rerate] {i}/{len(todo)} @ {rate:.2f} rows/s — "
                f"{dict(counts)}\n"
            )

    out.close()
    sys.stderr.write(f"\nDONE: {dict(counts)} in {time.time()-start:.0f}s\n")


if __name__ == "__main__":
    main()
