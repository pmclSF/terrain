#!/usr/bin/env python3
"""
Validate boundary-record TP/FP classifications using Claude Code's `claude` CLI.

This runs against your Claude subscription (no API key needed). Output is a
JSONL file with each record annotated with `_verdict` from Claude.

Resumable: re-running picks up where it left off via a per-record content hash.

Usage:
    python scripts/validate_corpus_claude.py
        --input  tier-4/sample-stratified.jsonl
        --output tier-4/llm-rated-validation.jsonl
        --max    500     # default: rate at most 500 records
        --batch  1       # records per claude call; default 1
        --concurrency 1  # parallel claude calls; default 1 (sequential)

Tested with: claude CLI 2.x in print mode (`claude -p`).
"""

import argparse
import concurrent.futures
import hashlib
import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path


PROMPT_TEMPLATE = """\
You are validating boundary detection records emitted by a static AI/ML code
corpus harvester. Each record claims that a detector fired on a source file
(or a manifest, or a derived edge). You must classify each as TP (true
positive — the detector correctly identified a real instance of the thing it
claims to detect) or FP (false positive — the detector mis-fired on
something unrelated like a docstring, comment, test fixture, vendor copy, or
a string literal mentioning the symbol).

Edge-type semantics (what counts as TP):

  src_calls_llm                 — file genuinely calls or imports an LLM SDK in
                                  production code, not in a string/comment.
  src_uses_ml                   — file genuinely imports an ML library
                                  (sklearn, torch, tensorflow, etc.) for use.
  src_trains_model              — file contains an actual training-loop call
                                  (.fit(), Trainer(), loss.backward(), etc.)
  src_loads_model               — file loads a serialized model (torch.load,
                                  joblib.load, from_pretrained, etc.)
  src_tracks_metric             — file imports/uses a metric tracker (mlflow,
                                  wandb, tensorboard).
  src_evaluates_model           — file uses an evaluation framework
                                  (promptfoo, deepeval, sklearn.metrics).
  src_sets_seed                 — file sets reproducibility seed
                                  (torch.manual_seed, np.random.seed, etc.).
  src_splits_data               — file performs a train/test split
                                  (train_test_split, KFold, etc.).
  src_checkpoints_model         — file emits a checkpoint (ModelCheckpoint,
                                  save_pretrained, log_model).
  src_declares_hyperparam       — file declares a training hyperparameter
                                  (lr=, batch_size=, epochs=).
  src_references_prompt         — file IS an AI prompt artifact (yaml/json/j2
                                  prompt template, not arbitrary config).
  surface_missing_eval          — derived: file genuinely calls an LLM but the
                                  repo has no eval framework imported.
  train_script_missing_tracker  — derived: file is genuinely a training script
                                  but the repo has no metric tracker.
  train_script_missing_seed     — derived: training file without seed set.
  train_script_missing_checkpoint — derived: training file without checkpoint.

A record is FP when the evidence line is inside a docstring, comment, test
fixture path, vendor copy, a string literal that just mentions the library
by name, or any path-noise the harvester should have filtered.

RECORD TO CLASSIFY:

  edge_type: {edge_type}
  symbol:    {dst_symbol}
  repo:      {repo}
  src.path:  {src_path}
  dst.path:  {dst_path}
  dst.kind:  {dst_kind}
  evidence:  {evidence}

Respond with EXACTLY one JSON object, no markdown fences, no prose around it:
{{"verdict":"TP","reason":"<one short sentence>"}}

or "FP", or "UNCERTAIN" if you genuinely can't tell from the record alone.
"""


def record_id(rec: dict) -> str:
    """Stable hash so resumes skip already-rated rows."""
    key = "|".join([
        rec.get("repo", ""),
        rec.get("edge_type", ""),
        (rec.get("src") or {}).get("path", ""),
        (rec.get("dst") or {}).get("path", ""),
        (rec.get("dst") or {}).get("symbol", ""),
        (rec.get("evidence") or "")[:120],
    ])
    return hashlib.sha1(key.encode()).hexdigest()[:16]


def already_rated(out_path: Path) -> set:
    seen = set()
    if not out_path.exists():
        return seen
    with out_path.open() as f:
        for line in f:
            try:
                r = json.loads(line)
                rid = r.get("_record_id")
                if rid:
                    seen.add(rid)
            except json.JSONDecodeError:
                continue
    return seen


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
    """Best-effort extract of a single JSON verdict object from Claude output."""
    if not raw:
        return {"verdict": "PARSE_ERROR", "reason": "empty response"}
    # Try clean JSON first
    raw_strip = raw.strip()
    for cand in (raw_strip,):
        try:
            obj = json.loads(cand)
            if isinstance(obj, dict) and "verdict" in obj:
                return obj
        except json.JSONDecodeError:
            pass
    # Look for first JSON-looking object containing "verdict"
    m = JSON_RE.search(raw)
    if m:
        try:
            return json.loads(m.group(0))
        except json.JSONDecodeError:
            pass
    return {
        "verdict": "PARSE_ERROR",
        "reason": "could not extract JSON",
        "raw_head": raw[:240],
    }


def call_claude(prompt: str, timeout: int = 90) -> dict:
    """Invoke claude CLI in print mode; return parsed verdict dict."""
    try:
        proc = subprocess.run(
            ["claude", "-p", prompt],
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return {"verdict": "TIMEOUT", "reason": f"claude exceeded {timeout}s"}
    except FileNotFoundError:
        sys.exit("ERROR: `claude` CLI not found in PATH. Install Claude Code first.")
    if proc.returncode != 0:
        return {
            "verdict": "CLI_ERROR",
            "reason": f"exit={proc.returncode}",
            "stderr_head": (proc.stderr or "")[:240],
        }
    return parse_verdict(proc.stdout)


def rate_one(rec: dict) -> dict:
    prompt = build_prompt(rec)
    return call_claude(prompt)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--input", default="tier-4/sample-stratified.jsonl",
                    help="JSONL of boundary records to rate")
    ap.add_argument("--output", default="tier-4/llm-rated-validation.jsonl",
                    help="JSONL output (resumable; one verdict per input record)")
    ap.add_argument("--max", type=int, default=500,
                    help="max records to rate in this run (default: 500)")
    ap.add_argument("--concurrency", type=int, default=1,
                    help="parallel claude calls (default: 1; raise cautiously)")
    ap.add_argument("--progress-every", type=int, default=10,
                    help="print progress every N records")
    args = ap.parse_args()

    in_path = Path(args.input)
    out_path = Path(args.output)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    if not in_path.exists():
        sys.exit(f"input not found: {in_path}")

    seen = already_rated(out_path)
    sys.stderr.write(f"[validate] {len(seen)} records already rated; resuming\n")

    # Build the worklist (skip already-rated)
    todo = []
    with in_path.open() as f:
        for line in f:
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            rid = record_id(rec)
            if rid in seen:
                continue
            rec["_record_id"] = rid
            todo.append(rec)
            if len(todo) >= args.max:
                break

    sys.stderr.write(f"[validate] {len(todo)} records to rate this run\n")
    if not todo:
        sys.stderr.write("[validate] nothing to do.\n")
        return

    start = time.time()
    out_f = out_path.open("a")
    n_done = 0
    n_tp = n_fp = n_unc = n_err = 0

    def handle_result(rec, verdict):
        nonlocal n_done, n_tp, n_fp, n_unc, n_err
        rec["_verdict"] = verdict
        out_f.write(json.dumps(rec) + "\n")
        out_f.flush()
        v = (verdict or {}).get("verdict", "")
        if v == "TP":
            n_tp += 1
        elif v == "FP":
            n_fp += 1
        elif v == "UNCERTAIN":
            n_unc += 1
        else:
            n_err += 1
        n_done += 1
        if n_done % args.progress_every == 0:
            elapsed = time.time() - start
            rate = n_done / elapsed if elapsed > 0 else 0
            sys.stderr.write(
                f"[validate] {n_done}/{len(todo)} done "
                f"(TP={n_tp} FP={n_fp} UNCERTAIN={n_unc} ERR={n_err}) "
                f"@ {rate:.2f}/s\n"
            )

    if args.concurrency <= 1:
        for rec in todo:
            verdict = rate_one(rec)
            handle_result(rec, verdict)
    else:
        with concurrent.futures.ThreadPoolExecutor(
            max_workers=args.concurrency
        ) as pool:
            fut_to_rec = {pool.submit(rate_one, rec): rec for rec in todo}
            for fut in concurrent.futures.as_completed(fut_to_rec):
                rec = fut_to_rec[fut]
                try:
                    verdict = fut.result()
                except Exception as exc:
                    verdict = {"verdict": "EXCEPTION", "reason": str(exc)}
                handle_result(rec, verdict)

    out_f.close()
    elapsed = time.time() - start
    sys.stderr.write(
        f"[validate] DONE: {n_done} rated in {elapsed:.0f}s "
        f"(TP={n_tp} FP={n_fp} UNCERTAIN={n_unc} ERR={n_err})\n"
    )
    if n_done:
        sys.stderr.write(
            f"[validate] post-rating precision (TP / (TP+FP)) = "
            f"{n_tp/max(1, n_tp+n_fp):.1%}\n"
        )


if __name__ == "__main__":
    main()
