#!/usr/bin/env python3
"""
Phase B.2 — Truncation-sensitivity test.

~11% of n=250 verdicts contain rationale language about excerpt truncation
("excerpt truncated", "cannot see", "not visible", "snippet"). If those
verdicts flip when re-rated with FULL file content, the precision math is
contaminated.

Method:
  1. Find rows whose verdict reason mentions truncation/visibility issues
  2. Sample 25 of them
  3. For each: clone the repo (shallow), read the FULL file
  4. Re-rate with Claude using full file context (up to 20K chars)
  5. Compare new verdict to original

If >10% of verdicts flip, truncation-tainted rows must be quarantined
or re-rated before being trusted for ship/no-ship decisions.
"""

from __future__ import annotations
import json
import random
import re
import shutil
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path


SEED = 73
N_SAMPLE = 25
TIMEOUT = 180
WORKDIR = Path("/tmp/b2-truncation")
MAX_FILE_CHARS = 20000  # cap full-file content to ~5K tokens


TRUNCATION_PATTERNS = re.compile(
    r"(excerpt (does not|doesn't|cannot|can't)|"
    r"truncat(ed|ion)|"
    r"not visible|cannot see|can't see|"
    r"not shown|snippet|limited (visibility|to)|"
    r"cannot (verify|confirm|determine)|"
    r"unable to (verify|confirm|see))",
    re.IGNORECASE)


PROMPT_TPL = """You are a senior software engineer reviewing a static-analysis finding produced by a code-quality tool.

Detector: {rule_id}
Severity: {severity}
Title: {title}
Description: {description}
Repository: {repo}
File: {file}
Symbol: {symbol}
Evidence: {evidence}

Full file content:
```
{full_file}
```

Is this finding a TRUE POSITIVE (real problem worth fixing) or FALSE POSITIVE (noise)?

Consider the FULL file content above when judging — you can now see what was hidden in the previous excerpt.

Respond ONLY in JSON: {{"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "<one line>"}}"""


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


def clone_if_needed(repo: str) -> Path | None:
    """Return path to clone, cloning if not present. Returns None on failure."""
    safe_name = repo.replace("/", "__")
    target = WORKDIR / safe_name
    if (target / ".git").exists():
        return target
    parts = repo.split("__", 1)
    if len(parts) != 2:
        # fallback: try as owner/name
        parts = repo.split("/", 1) if "/" in repo else None
        if not parts or len(parts) != 2:
            return None
    owner, name = parts
    url = f"https://github.com/{owner}/{name}.git"
    try:
        subprocess.run(
            ["git", "clone", "--depth=1", "--filter=blob:limit=512k",
             "--no-tags", "-q", url, str(target)],
            check=True, timeout=120, capture_output=True,
        )
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
        shutil.rmtree(target, ignore_errors=True)
        return None
    return target


def read_file_safe(repo_path: Path, rel: str) -> str | None:
    full = repo_path / rel
    if not full.exists() or not full.is_file():
        return None
    try:
        content = full.read_text(errors="replace")
    except (OSError, UnicodeDecodeError):
        return None
    if len(content) > MAX_FILE_CHARS:
        # head + tail to preserve both ends for very long files
        head = content[:MAX_FILE_CHARS // 2]
        tail = content[-(MAX_FILE_CHARS // 2):]
        content = f"{head}\n\n[...truncated middle for length...]\n\n{tail}"
    return content


def main():
    rows = load_merged()

    # Find truncation-tainted rows
    tainted = [r for r in rows
               if TRUNCATION_PATTERNS.search(
                   (r.get("_verdict") or {}).get("reason", ""))
               and (r.get("_verdict") or {}).get("verdict") in ("TP", "FP")]
    sys.stderr.write(f"[B.2] {len(tainted)} truncation-tainted rows in corpus "
                     f"({len(tainted) / len(rows) * 100:.1f}% of all)\n")

    rng = random.Random(SEED)
    if len(tainted) >= N_SAMPLE:
        sample = rng.sample(tainted, N_SAMPLE)
    else:
        sample = tainted
    sys.stderr.write(f"[B.2] sampling {len(sample)} for re-rate\n")

    WORKDIR.mkdir(parents=True, exist_ok=True)

    out_rows = []
    failures = 0
    start = time.time()
    for i, r in enumerate(sample, 1):
        repo = r.get("repo", "")
        repo_path = clone_if_needed(repo)
        if not repo_path:
            failures += 1
            sys.stderr.write(f"[B.2] {i}/{len(sample)} FAIL clone {repo}\n")
            continue
        file_content = read_file_safe(repo_path, r.get("file", ""))
        if not file_content:
            failures += 1
            sys.stderr.write(f"[B.2] {i}/{len(sample)} FAIL read "
                             f"{repo}/{r.get('file', '')}\n")
            continue

        prompt = PROMPT_TPL.format(
            rule_id=r.get("rule_id", ""),
            severity=r.get("severity", "medium"),
            title=(r.get("title", "") or "")[:200],
            description=(r.get("description", "") or "")[:400],
            repo=repo,
            file=r.get("file", ""),
            symbol=r.get("symbol", ""),
            evidence=(r.get("evidence", "") or "")[:500],
            full_file=file_content,
        )
        new_verdict = call_claude(prompt)
        orig = (r.get("_verdict") or {}).get("verdict", "?")
        new = new_verdict.get("verdict", "?")
        out_rows.append({
            "rule_id": r.get("rule_id"),
            "repo": repo,
            "file": r.get("file"),
            "symbol": r.get("symbol"),
            "original_verdict": orig,
            "original_reason": (r.get("_verdict") or {}).get("reason", "")[:200],
            "new_verdict": new,
            "new_reason": new_verdict.get("reason", "")[:200],
            "file_size": len(file_content),
            "agree": orig == new,
        })
        if i % 5 == 0:
            elapsed = time.time() - start
            sys.stderr.write(f"[B.2] {i}/{len(sample)} done @ "
                             f"{i / elapsed * 60:.1f} rows/min\n")

    # Stats
    n = len(out_rows)
    if n == 0:
        print("ERROR: no rows re-rated successfully")
        return
    agree = sum(1 for r in out_rows if r["agree"])
    flip_rate = (n - agree) / n * 100

    # Confusion matrix
    pairs = Counter((r["original_verdict"], r["new_verdict"]) for r in out_rows)

    # Per-detector
    by_det = defaultdict(lambda: {"n": 0, "agree": 0})
    for r in out_rows:
        d = r["rule_id"]
        by_det[d]["n"] += 1
        if r["agree"]:
            by_det[d]["agree"] += 1

    print()
    print("=" * 90)
    print("Phase B.2 — Truncation-sensitivity test (re-rate with full file)")
    print("=" * 90)
    print()
    print(f"Truncation-tainted rows in corpus: {len(tainted)} "
          f"({len(tainted) / len(rows) * 100:.1f}%)")
    print(f"Rows re-rated: {n}")
    print(f"Clone/read failures: {failures}")
    print(f"Verdicts that flipped: {n - agree} ({flip_rate:.1f}%)")
    print()
    print("Verdict transitions (original -> new):")
    for (o, nv), c in sorted(pairs.items(), key=lambda kv: -kv[1]):
        print(f"  {o:>10s} -> {nv:>10s}: {c}")
    print()
    print("Per-detector flip rate:")
    for d, c in sorted(by_det.items()):
        if c["n"] > 0:
            agp = c["agree"] / c["n"] * 100
            print(f"  {d:30s}  n={c['n']}  agree={c['agree']}/{c['n']} "
                  f"({agp:.0f}%)")
    print()

    if flip_rate > 10:
        print("DECISIVE: Truncation-tainted verdicts are unreliable.")
        print(f"Recommend quarantining the {len(tainted)} tainted rows OR "
              "re-rating them with full file content before using for ship "
              "decisions.")
    else:
        print(f"OK: flip rate {flip_rate:.1f}% is within tolerance. "
              "Truncation language in rationale is not materially affecting "
              "verdict accuracy.")
    print()

    out = Path("tier-4/phase-b2-results.json")
    with out.open("w") as f:
        json.dump({
            "tainted_total": len(tainted),
            "tainted_pct_of_corpus": len(tainted) / len(rows) * 100,
            "n_rerated": n,
            "failures": failures,
            "flip_rate_pct": flip_rate,
            "transitions": {f"{o}->{n}": c for (o, n), c in pairs.items()},
            "per_detector": dict(by_det),
            "rows": out_rows,
        }, f, indent=2)
    print(f"Results: {out}")


if __name__ == "__main__":
    main()
