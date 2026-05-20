#!/usr/bin/env python3
"""
Validate Terrain's PRODUCTION DETECTORS (terrain insights / analyze output)
against Claude's judgment per finding.

This is one layer ABOVE the boundary-record validator. Boundary records are
intermediate signals; what customers see are the production detectors that
fire via `terrain insights`. This script:

  1. Picks a stratified sample of repos from the corpus
  2. Shallow-clones each
  3. Runs `terrain insights --json` on each
  4. Collects per-detector findings (rule_id + file + evidence)
  5. Samples N TP candidates and N TN candidates per detector
  6. Sends each to Claude CLI: "should rule_id have fired on this file?"
  7. Computes per-detector precision (TP/(TP+FP)) + emits report

Cost: $0 via Claude subscription. Disk: bounded by streaming clone+delete.

Usage:
    python3 scripts/validate_detectors_claude.py
        --repo-list tier-4/sample-repos.txt   # one owner__name per line
        --terrain-bin /usr/local/bin/terrain
        --output tier-4/detector-validation.jsonl
        --tp-per-detector 30
        --workdir /tmp/detector-validation
"""
import argparse
import hashlib
import json
import os
import random
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path
from collections import defaultdict


# --- Prompt to Claude ---

PROMPT_TEMPLATE = """\
You are validating findings emitted by Terrain, a static AI/ML code-quality
analyzer. For each finding, judge whether the detector correctly identified
a real instance (TP) or fired on something unrelated (FP).

DETECTOR RULE: {rule_id}
SEVERITY:      {severity}
TITLE:         {title}
DESCRIPTION:   {description}

REPO:          {repo}
FILE:          {file}
EVIDENCE:      {evidence}

A few lines of context from the file (HEAD revision):
{file_excerpt}

Respond with EXACTLY one JSON object, no markdown, no prose:
{{"verdict":"TP","reason":"<one short sentence>"}}

Use "TP" if the rule correctly identified the thing it claims to detect.
Use "FP" if the rule mis-fired (docstring, comment, test fixture, vendor,
wrong file shape, false negative inversion, etc.).
Use "UNCERTAIN" only if the record + excerpt are genuinely insufficient.
"""


# --- Terrain CLI helpers ---

def run_terrain_snapshot(terrain_bin: str, repo_path: Path,
                         timeout: int = 180) -> dict | None:
    """Run terrain analyze --write-snapshot and return the parsed snapshot.

    The public CLI (`terrain insights --json`, `terrain analyze --json`) only
    emits repo-level aggregates. The granular per-rule per-file data lives in
    .terrain/snapshots/latest.json, which terrain writes when invoked with
    --write-snapshot. We read it directly.
    """
    try:
        proc = subprocess.run(
            [terrain_bin, "analyze", "--write-snapshot", "--root", str(repo_path)],
            capture_output=True, text=True, timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return None
    except FileNotFoundError:
        sys.exit(f"ERROR: {terrain_bin} not found")
    if proc.returncode != 0:
        sys.stderr.write(f"[terrain] {repo_path.name}: exit={proc.returncode}\n")
        return None
    snapshot_path = repo_path / ".terrain" / "snapshots" / "latest.json"
    if not snapshot_path.exists():
        return None
    try:
        with snapshot_path.open() as f:
            return json.load(f)
    except (json.JSONDecodeError, OSError):
        return None


def extract_findings(snapshot: dict, repo: str) -> list[dict]:
    """Pull per-rule per-file findings from terrain snapshot.signals.

    Each entry in `snapshot.signals` is a per-file detector hit with:
      - type:            the rule id (e.g. 'uncoveredAISurface', 'untestedExport')
      - category:        bucket (ai / quality / coverage / ...)
      - severity:        high / medium / low
      - confidence:      float 0-1
      - location.file:   the file the rule fired on
      - location.symbol: the symbol (if applicable)
      - explanation:     human-readable evidence
      - suggestedAction: remediation hint
    """
    if not isinstance(snapshot, dict):
        return []
    signals = snapshot.get("signals", [])
    if not isinstance(signals, list):
        return []
    out = []
    for s in signals:
        if not isinstance(s, dict):
            continue
        rule = s.get("type")
        if not rule:
            continue
        loc = s.get("location") or {}
        out.append({
            "repo": repo,
            "rule_id": rule,
            "file": loc.get("file", ""),
            "symbol": loc.get("symbol", ""),
            "severity": s.get("severity", ""),
            "category": s.get("category", ""),
            "confidence": s.get("confidence"),
            "evidence_strength": s.get("evidenceStrength", ""),
            "evidence_source": s.get("evidenceSource", ""),
            "title": (s.get("explanation") or "")[:160],
            "description": s.get("explanation", ""),
            "evidence": (s.get("explanation") or "") + " | " +
                        (s.get("suggestedAction") or ""),
        })
    return out


# --- Sampling ---

def sample_per_detector(findings: list[dict], n_per: int,
                        skip_keys: set | None = None) -> list[dict]:
    by_rule = defaultdict(list)
    for f in findings:
        key = (f.get("repo", ""), f.get("rule_id", ""),
               f.get("file", ""), f.get("symbol", ""))
        if skip_keys and key in skip_keys:
            continue
        by_rule[f["rule_id"]].append(f)
    out = []
    for rule, items in by_rule.items():
        random.shuffle(items)
        out.extend(items[:n_per])
    return out


def load_skip_set(path: Path) -> set:
    """Read prior detector-validation.jsonl and return a set of already-rated keys."""
    keys = set()
    if not path.exists():
        return keys
    with path.open() as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            keys.add((row.get("repo", ""), row.get("rule_id", ""),
                      row.get("file", ""), row.get("symbol", "")))
    return keys


# --- Excerpt loader ---

def file_excerpt(repo_path: Path, rel_file: str, lines_around: int = 40) -> str:
    if not rel_file:
        return "(no file path)"
    full = repo_path / rel_file
    try:
        with full.open("r", errors="replace") as f:
            content = f.readlines()
    except (OSError, IsADirectoryError):
        return "(could not read file)"
    if len(content) <= lines_around * 2:
        return "".join(content)[:4000]
    head = "".join(content[:lines_around])
    tail = "".join(content[-lines_around:])
    return f"{head}\n... [truncated middle] ...\n{tail}"[:4000]


# --- Claude rating ---

JSON_RE = re.compile(r"\{[^{}]*\"verdict\"[^{}]*\}", re.DOTALL)


def parse_verdict(raw: str) -> dict:
    if not raw:
        return {"verdict": "PARSE_ERROR", "reason": "empty"}
    try:
        return json.loads(raw.strip())
    except json.JSONDecodeError:
        pass
    m = JSON_RE.search(raw)
    if m:
        try:
            return json.loads(m.group(0))
        except json.JSONDecodeError:
            pass
    return {"verdict": "PARSE_ERROR", "reason": "extract-fail",
            "raw_head": raw[:240]}


def call_claude(prompt: str, timeout: int = 90) -> dict:
    # subprocess.run rejects strings with embedded null bytes as command args;
    # source files occasionally contain \x00 (binaries misnamed, build outputs).
    prompt = prompt.replace("\x00", "")
    try:
        proc = subprocess.run(["claude", "-p", prompt],
                              capture_output=True, text=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        return {"verdict": "TIMEOUT", "reason": f"claude >{timeout}s"}
    except FileNotFoundError:
        sys.exit("ERROR: claude CLI not in PATH")
    if proc.returncode != 0:
        return {"verdict": "CLI_ERROR", "reason": f"exit={proc.returncode}"}
    return parse_verdict(proc.stdout)


# --- Main ---

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo-list", required=True,
                    help="file with one owner__name per line")
    ap.add_argument("--terrain-bin", default="/usr/local/bin/terrain")
    ap.add_argument("--workdir", default="/tmp/detector-validation",
                    help="streaming workdir for clones (created+cleaned)")
    ap.add_argument("--output", default="tier-4/detector-validation.jsonl")
    ap.add_argument("--tp-per-detector", type=int, default=30,
                    help="findings to sample per detector across all repos")
    ap.add_argument("--max-repos", type=int, default=50)
    ap.add_argument("--depth", type=int, default=1)
    ap.add_argument("--progress-every", type=int, default=10)
    ap.add_argument("--skip-already-rated", default="",
                    help="path to a prior detector-validation.jsonl whose "
                         "(repo, rule_id, file, symbol) keys should be "
                         "excluded from this run's sample")
    ap.add_argument("--resume-from", default="",
                    help="path to an in-progress output JSONL from a crashed "
                         "run; keys in this file are skipped during stage 3 "
                         "(preserves the original sample sequence) and output "
                         "is opened in append mode")
    ap.add_argument("--findings-cache", default="",
                    help="path to dump/read Stage 1 findings JSONL. If the file "
                         "exists, Stage 1 (clone+scan 500 repos) is SKIPPED and "
                         "findings are loaded from cache. If it doesn't exist, "
                         "Stage 1 runs normally and dumps findings to this path "
                         "on completion — so a Stage 3 crash can be resumed "
                         "without re-cloning. Recommended for any --max-repos > 50.")
    args = ap.parse_args()

    repo_list_path = Path(args.repo_list)
    if not repo_list_path.exists():
        sys.exit(f"repo list not found: {repo_list_path}")
    work_root = Path(args.workdir)
    work_root.mkdir(parents=True, exist_ok=True)
    out_path = Path(args.output)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    repos = []
    with repo_list_path.open() as f:
        for line in f:
            n = line.strip()
            if n and not n.startswith("#"):
                repos.append(n)
    random.seed(42)
    random.shuffle(repos)
    repos = repos[: args.max_repos]
    sys.stderr.write(f"[detector-validate] {len(repos)} repos to scan\n")

    # ─── Stage 1: clone, run terrain, collect findings, delete ───────────
    # If --findings-cache points to an existing file, skip Stage 1 entirely
    # and load findings from cache. This protects against Stage 3 crashes
    # forcing a full re-scan: a finished Stage 1 always writes the cache.
    cache_path = Path(args.findings_cache) if args.findings_cache else None
    all_findings = []
    if cache_path and cache_path.exists():
        sys.stderr.write(
            f"[detector-validate] loading findings from cache: {cache_path}\n"
        )
        with cache_path.open() as cf:
            for line in cf:
                line = line.strip()
                if line:
                    try:
                        all_findings.append(json.loads(line))
                    except json.JSONDecodeError:
                        continue
        sys.stderr.write(
            f"[detector-validate] cache loaded: {len(all_findings)} findings\n"
        )
    else:
        for i, repo in enumerate(repos, 1):
            parts = repo.split("__", 1)
            if len(parts) != 2:
                continue
            owner, name = parts
            url = f"https://github.com/{owner}/{name}.git"
            target = work_root / repo
            try:
                subprocess.run(
                    ["git", "clone", f"--depth={args.depth}",
                     "--filter=blob:limit=512k", "--no-tags", "-q", url, str(target)],
                    check=True, timeout=300, capture_output=True,
                )
            except (subprocess.TimeoutExpired, subprocess.CalledProcessError) as e:
                sys.stderr.write(f"[detector-validate] FAIL clone {repo}\n")
                shutil.rmtree(target, ignore_errors=True)
                continue
            snapshot = run_terrain_snapshot(args.terrain_bin, target)
            findings = extract_findings(snapshot or {}, repo)
            # Materialize file excerpts BEFORE we delete the clone
            for f in findings:
                f["_file_excerpt"] = file_excerpt(target, f.get("file", ""))
                f["_repo_path"] = str(target)
            all_findings.extend(findings)
            shutil.rmtree(target, ignore_errors=True)
            if i % args.progress_every == 0:
                sys.stderr.write(
                    f"[detector-validate] {i}/{len(repos)} scanned, "
                    f"{len(all_findings)} total findings so far\n"
                )
        if cache_path:
            cache_path.parent.mkdir(parents=True, exist_ok=True)
            with cache_path.open("w") as cf:
                for f in all_findings:
                    cf.write(json.dumps(f) + "\n")
            sys.stderr.write(
                f"[detector-validate] Stage 1 cache written: {cache_path} "
                f"({len(all_findings)} findings)\n"
            )
    sys.stderr.write(f"[detector-validate] {len(all_findings)} total findings "
                     f"across {len(repos)} repos\n")
    if not all_findings:
        sys.exit("no findings collected — check terrain insights output shape")

    # ─── Stage 2: stratified sample per detector ────────────────────────
    skip_keys = set()
    if args.skip_already_rated:
        skip_keys = load_skip_set(Path(args.skip_already_rated))
        sys.stderr.write(
            f"[detector-validate] skip-set: {len(skip_keys)} already-rated "
            f"keys from {args.skip_already_rated}\n"
        )
    sample = sample_per_detector(all_findings, args.tp_per_detector, skip_keys)
    sys.stderr.write(f"[detector-validate] sampling {len(sample)} for Claude\n")

    # ─── Stage 3: rate each sample via Claude ───────────────────────────
    resume_keys = set()
    if args.resume_from:
        resume_keys = load_skip_set(Path(args.resume_from))
        sys.stderr.write(
            f"[detector-validate] resume: {len(resume_keys)} already-rated "
            f"rows in {args.resume_from} will be skipped\n"
        )
    out_f = out_path.open("a" if resume_keys else "w")
    n_tp = n_fp = n_unc = n_skip = 0
    start = time.time()
    for i, f in enumerate(sample, 1):
        key = (f.get("repo", ""), f.get("rule_id", ""),
               f.get("file", ""), f.get("symbol", ""))
        if key in resume_keys:
            n_skip += 1
            continue
        prompt = PROMPT_TEMPLATE.format(
            rule_id=f["rule_id"],
            severity=f.get("severity", "(unknown)"),
            title=f.get("title", "(no title)"),
            description=(f.get("description", "(no description)") or "")[:400],
            repo=f["repo"],
            file=f.get("file", "(no file)"),
            evidence=(f.get("evidence", "(no evidence)") or "")[:500],
            file_excerpt=f.get("_file_excerpt", "")[:2500],
        )
        verdict = call_claude(prompt)
        rec = dict(f)
        # Keep _file_excerpt in output so strict inter-rater reliability tests
        # can re-rate with identical context. Trade-off: ~2.5KB per row of
        # file content increases output file size by ~7-8x. Worth it: without
        # this, future B.1 tests have no way to reproduce the original prompt.
        # If size is an issue, downstream tools can strip _file_excerpt.
        rec["_verdict"] = verdict
        out_f.write(json.dumps(rec) + "\n")
        out_f.flush()
        v = verdict.get("verdict", "")
        if v == "TP":
            n_tp += 1
        elif v == "FP":
            n_fp += 1
        elif v == "UNCERTAIN":
            n_unc += 1
        if i % 10 == 0:
            rate = i / max(1, time.time() - start)
            sys.stderr.write(f"[detector-validate] rated {i}/{len(sample)} "
                             f"TP={n_tp} FP={n_fp} UNC={n_unc} @ {rate:.1f}/s\n")
    out_f.close()
    sys.stderr.write(
        f"\n[detector-validate] DONE: {len(sample)} rated\n"
        f"  TP={n_tp} FP={n_fp} UNCERTAIN={n_unc}\n"
        f"  precision = {n_tp / max(1, n_tp + n_fp):.0%}\n"
    )

    # ─── Stage 4: per-detector breakdown ─────────────────────────────────
    by_rule = defaultdict(lambda: {"tp": 0, "fp": 0, "unc": 0, "n": 0})
    with out_path.open() as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            rule = r["rule_id"]
            v = (r.get("_verdict") or {}).get("verdict", "")
            by_rule[rule]["n"] += 1
            if v == "TP":
                by_rule[rule]["tp"] += 1
            elif v == "FP":
                by_rule[rule]["fp"] += 1
            elif v == "UNCERTAIN":
                by_rule[rule]["unc"] += 1
    sys.stderr.write("\n=== Per-detector precision ===\n")
    rows = sorted(by_rule.items(),
                  key=lambda kv: -(kv[1]["tp"] / max(1, kv[1]["tp"] + kv[1]["fp"])))
    for rule, c in rows:
        denom = c["tp"] + c["fp"]
        p = (c["tp"] / denom) if denom else 0
        sys.stderr.write(
            f"  {rule:50}  TP={c['tp']:>3} FP={c['fp']:>3} UNC={c['unc']:>2} "
            f"n={c['n']:>3}  precision={p:.0%}\n"
        )


if __name__ == "__main__":
    main()
