#!/usr/bin/env python3
"""
Cross-check Claude's verdicts (from validate_corpus_claude.py) against
GPT-4o (or gpt-4o-mini) to measure rater bias.

For each sampled record, GPT-4o sees the SAME prompt Claude saw and emits
its own verdict. Output: per-record both verdicts side-by-side + agreement
stats. Aim: detect whether Claude is systematically over-classifying TPs.

Usage:
    OPENAI_API_KEY=sk-... python3 scripts/validate_corpus_openai_check.py
        --input tier-4/llm-rated-validation.jsonl
        --output tier-4/openai-crosscheck.jsonl
        --max 200
        --model gpt-4o            # or gpt-4o-mini for ~10x cheaper

Cost rough estimate (200 records, ~500 prompt + 100 response tokens):
    gpt-4o-mini: $0.03
    gpt-4o:      $0.45
"""
import argparse
import hashlib
import json
import os
import re
import sys
import time
from pathlib import Path
from urllib import request as urlrequest, error as urlerror


PROMPT_TEMPLATE = """\
You are validating boundary detection records emitted by a static AI/ML code
corpus harvester. Each record claims that a detector fired on a source file.
Classify each as TP (true positive — detector correctly identified a real
instance) or FP (false positive — detector mis-fired on docstring, comment,
test fixture, vendor copy, or string-literal mention).

Edge-type semantics:
  src_calls_llm                 — file genuinely calls/imports an LLM SDK
  src_uses_ml                   — file genuinely imports an ML library
  src_trains_model              — actual training-loop call
  src_loads_model               — file loads a serialized model
  src_tracks_metric             — uses metric tracker (mlflow/wandb)
  src_evaluates_model           — uses eval framework
  src_sets_seed                 — sets reproducibility seed
  src_splits_data               — train/test split
  src_checkpoints_model         — emits checkpoint
  src_declares_hyperparam       — declares hyperparameter
  src_references_prompt         — file is a prompt artifact (yaml/j2/json)
  surface_missing_eval          — derived: real LLM call, no eval framework
  train_script_missing_tracker  — derived: real training, no tracker
  train_script_missing_seed     — derived: training without seed
  train_script_missing_checkpoint — derived: training without checkpoint

RECORD:
  edge_type: {edge_type}
  symbol:    {dst_symbol}
  repo:      {repo}
  src.path:  {src_path}
  dst.path:  {dst_path}
  dst.kind:  {dst_kind}
  evidence:  {evidence}

Respond with EXACTLY one JSON object, no markdown, no prose:
{{"verdict":"TP","reason":"<one short sentence>"}}
or "FP", or "UNCERTAIN" if genuinely cannot tell.
"""


def build_prompt(rec: dict) -> str:
    return PROMPT_TEMPLATE.format(
        edge_type=rec.get("edge_type", ""),
        dst_symbol=(rec.get("dst") or {}).get("symbol", "(none)"),
        repo=rec.get("repo", ""),
        src_path=(rec.get("src") or {}).get("path", ""),
        dst_path=(rec.get("dst") or {}).get("path", ""),
        dst_kind=(rec.get("dst") or {}).get("kind", ""),
        evidence=(rec.get("evidence") or "(none)")[:500],
    )


JSON_RE = re.compile(r"\{[^{}]*\"verdict\"[^{}]*\}", re.DOTALL)


def parse_verdict(raw: str) -> dict:
    if not raw:
        return {"verdict": "PARSE_ERROR", "reason": "empty"}
    try:
        obj = json.loads(raw.strip())
        if isinstance(obj, dict) and "verdict" in obj:
            return obj
    except json.JSONDecodeError:
        pass
    m = JSON_RE.search(raw)
    if m:
        try:
            return json.loads(m.group(0))
        except json.JSONDecodeError:
            pass
    return {"verdict": "PARSE_ERROR", "reason": "extract-fail", "raw_head": raw[:200]}


def call_openai(prompt: str, model: str, api_key: str, timeout: int = 60) -> dict:
    body = json.dumps({
        "model": model,
        "messages": [
            {"role": "system", "content": "You output only a single JSON object on one line."},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0,
        "max_tokens": 200,
    }).encode("utf-8")
    req = urlrequest.Request(
        "https://api.openai.com/v1/chat/completions",
        data=body,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urlrequest.urlopen(req, timeout=timeout) as resp:
            payload = json.loads(resp.read())
    except urlerror.HTTPError as e:
        body_text = e.read().decode("utf-8", errors="replace")[:240]
        return {"verdict": "API_ERROR", "reason": f"HTTP {e.code}", "body": body_text}
    except Exception as exc:
        return {"verdict": "API_ERROR", "reason": str(exc)}
    try:
        content = payload["choices"][0]["message"]["content"]
    except (KeyError, IndexError):
        return {"verdict": "PARSE_ERROR", "reason": "no content", "payload": str(payload)[:200]}
    return parse_verdict(content)


def stratified_sample(records: list, n_target: int) -> list:
    """Balance sample across Claude's verdict buckets so FPs get cross-checked too."""
    import random
    random.seed(42)
    buckets = {}
    for r in records:
        v = (r.get("_verdict") or {}).get("verdict", "UNKNOWN")
        buckets.setdefault(v, []).append(r)
    out = []
    per_bucket = max(1, n_target // max(1, len(buckets)))
    for v, recs in buckets.items():
        out.extend(random.sample(recs, min(per_bucket, len(recs))))
    # If under target, top up randomly across all
    if len(out) < n_target:
        rest = [r for r in records if r not in out]
        out.extend(random.sample(rest, min(n_target - len(out), len(rest))))
    return out[:n_target]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--input", default="tier-4/llm-rated-validation.jsonl")
    ap.add_argument("--output", default="tier-4/openai-crosscheck.jsonl")
    ap.add_argument("--max", type=int, default=200)
    ap.add_argument("--model", default="gpt-4o",
                    help="gpt-4o (~$0.45/200) or gpt-4o-mini (~$0.03/200)")
    ap.add_argument("--progress-every", type=int, default=20)
    args = ap.parse_args()

    api_key = os.environ.get("OPENAI_API_KEY")
    if not api_key:
        sys.exit("ERROR: set OPENAI_API_KEY environment variable.")

    in_path = Path(args.input)
    out_path = Path(args.output)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    if not in_path.exists():
        sys.exit(f"input not found: {in_path}")

    # Load Claude-rated records
    claude_rated = []
    with in_path.open() as f:
        for line in f:
            try:
                claude_rated.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    sys.stderr.write(f"[openai-check] loaded {len(claude_rated)} Claude-rated records\n")

    # Resume support: skip already-cross-checked
    done_ids = set()
    if out_path.exists():
        with out_path.open() as f:
            for line in f:
                try:
                    r = json.loads(line)
                    done_ids.add(r.get("_record_id"))
                except json.JSONDecodeError:
                    continue
        sys.stderr.write(f"[openai-check] {len(done_ids)} already cross-checked, resuming\n")

    # Stratified sample
    pool = [r for r in claude_rated if r.get("_record_id") not in done_ids]
    sample = stratified_sample(pool, args.max)
    sys.stderr.write(f"[openai-check] sampling {len(sample)} records (stratified by Claude verdict)\n")

    out_f = out_path.open("a")
    n = 0
    agree = disagree = 0
    by_verdict_pair = {}
    start = time.time()

    for rec in sample:
        prompt = build_prompt(rec)
        openai_verdict = call_openai(prompt, args.model, api_key)
        claude_verdict = (rec.get("_verdict") or {}).get("verdict", "UNKNOWN")
        oai_v = openai_verdict.get("verdict", "ERR")

        rec_out = dict(rec)
        rec_out["_openai_verdict"] = openai_verdict
        rec_out["_openai_model"] = args.model
        rec_out["_agreement"] = (claude_verdict == oai_v)
        out_f.write(json.dumps(rec_out) + "\n")
        out_f.flush()

        if claude_verdict == oai_v:
            agree += 1
        else:
            disagree += 1
        key = f"{claude_verdict} vs {oai_v}"
        by_verdict_pair[key] = by_verdict_pair.get(key, 0) + 1

        n += 1
        if n % args.progress_every == 0:
            elapsed = time.time() - start
            rate = n / elapsed if elapsed else 0
            sys.stderr.write(
                f"[openai-check] {n}/{len(sample)}  agree={agree} disagree={disagree}  "
                f"({100*agree/n:.0f}% agreement)  @ {rate:.1f}/s\n"
            )
        time.sleep(0.1)  # gentle pacing

    out_f.close()
    elapsed = time.time() - start
    sys.stderr.write(f"\n[openai-check] DONE: {n} cross-checked in {elapsed:.0f}s\n\n")

    sys.stderr.write("=== Verdict-pair breakdown (Claude vs OpenAI) ===\n")
    for k, v in sorted(by_verdict_pair.items(), key=lambda x: -x[1]):
        marker = "✓" if k.split(" vs ")[0] == k.split(" vs ")[1] else "✗"
        sys.stderr.write(f"  {marker} {k:30} {v}\n")

    # Compute the three useful agreement metrics
    pair_count = lambda c, o: by_verdict_pair.get(f"{c} vs {o}", 0)
    tp_tp = pair_count("TP", "TP")
    fp_fp = pair_count("FP", "FP")
    tp_fp = pair_count("TP", "FP")
    fp_tp = pair_count("FP", "TP")
    unc_tp = pair_count("UNCERTAIN", "TP")
    unc_fp = pair_count("UNCERTAIN", "FP")
    claude_tp = tp_tp + tp_fp
    claude_fp = fp_fp + fp_tp
    claude_unc = unc_tp + unc_fp
    both_committed = tp_tp + fp_fp + tp_fp + fp_tp

    sys.stderr.write("\n=== Agreement metrics ===\n")
    sys.stderr.write(
        f"  Naïve agreement (any UNCERTAIN counts as 'disagree'): "
        f"{agree}/{n} = {100*agree/max(1,n):.1f}%  ← MISLEADING when UNCERTAIN is common\n"
    )
    if both_committed > 0:
        substantive = tp_tp + fp_fp
        sys.stderr.write(
            f"  Substantive agreement (TP/FP direction, ignoring UNCERTAIN): "
            f"{substantive}/{both_committed} = {100*substantive/both_committed:.1f}%\n"
        )
    if claude_tp > 0:
        sys.stderr.write(
            f"  Claude TP -> OpenAI confirms: "
            f"{tp_tp}/{claude_tp} = {100*tp_tp/claude_tp:.1f}%  "
            f"(Claude's TPs are reliable if this is high)\n"
        )
    if claude_fp > 0:
        sys.stderr.write(
            f"  Claude FP -> OpenAI confirms: "
            f"{fp_fp}/{claude_fp} = {100*fp_fp/claude_fp:.1f}%  "
            f"(if low, OpenAI is more lenient than Claude on FPs)\n"
        )
    if claude_unc > 0:
        sys.stderr.write(
            f"  Claude UNCERTAIN -> OpenAI confidently TP: "
            f"{unc_tp}/{claude_unc} = {100*unc_tp/claude_unc:.1f}%  "
            f"(salvageable Claude punts)\n"
        )


if __name__ == "__main__":
    main()
