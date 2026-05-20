#!/usr/bin/env python3
"""
scripts/canary-driver.py — drive the canary set against the local terrain binary.

Reads harness/canary/canary-set.yaml and walks each PR entry:
  - sealed entry (head_sha + base_sha populated): clone, checkout, run terrain,
    diff findings against expected_findings + expected_non_findings, report.
  - draft entry: print status and skip.
  - HAND-SEED-REQUIRED entry: print seed instructions and skip.

This is the harness scaffold. The actual run logic for sealed entries is stubbed
out below — once entries are sealed (the first `make canary` run after hand-
resolving ground truth), the stub becomes the real run.

Stdlib-only: uses a shallow line-based parse keyed off the canary-set.yaml
structure (id / repo / pr_number / head_sha / base_sha / category). Anything
more complex would warrant PyYAML; for the current file shape this is enough.

Exit codes:
  0 — all sealed entries pass (or no sealed entries yet — full draft)
  1 — at least one expected TP missed
  2 — at least one expected non-finding fired
  3 — set is not sealed (warning only; not a CI failure)
  4 — usage / config error
"""

import argparse
import json
import re
import sys
from pathlib import Path


def parse_canary(path):
    """Parse harness/canary/canary-set.yaml using only stdlib. Returns list of dicts with
    id / repo / pr_number / head_sha / base_sha / category / notes (raw text)."""
    entries = []
    current = None
    current_key_indent = None
    notes_lines = None
    notes_indent = None

    line_re_id = re.compile(r"^(\s*)- id:\s*(canary-\S+)\s*$")
    line_re_kv = re.compile(r"^(\s+)([\w_]+):\s*(.*)$")

    for raw in path.read_text().splitlines():
        # End-of-block detection for notes:
        if notes_lines is not None:
            stripped = raw.rstrip()
            indent = len(raw) - len(raw.lstrip(" "))
            if not stripped:
                notes_lines.append("")
                continue
            if indent > notes_indent:
                notes_lines.append(raw[notes_indent + 2 :])
                continue
            # left the block
            current["notes"] = "\n".join(notes_lines).strip()
            notes_lines = None
            notes_indent = None

        m = line_re_id.match(raw)
        if m:
            if current is not None:
                entries.append(current)
            current = {"id": m.group(2)}
            current_key_indent = len(m.group(1)) + 2  # contents indent = bullet indent + 2
            continue

        if current is None:
            continue

        m = line_re_kv.match(raw)
        if not m:
            continue
        indent = len(m.group(1))
        if indent != current_key_indent:
            continue
        key = m.group(2)
        value = m.group(3).strip()

        if key in ("notes", "why_canary"):
            # multi-line block (we treat `|` and plain similarly)
            notes_lines = []
            notes_indent = indent
            continue

        if key in {"id", "repo", "pr_url", "merged_at", "category", "head_sha", "base_sha"}:
            current[key] = value if value not in ("null", "") else None
        elif key == "pr_number":
            try:
                current[key] = int(value) if value not in ("null", "") else None
            except ValueError:
                current[key] = value

    if current is not None:
        if notes_lines is not None:
            current["notes"] = "\n".join(notes_lines).strip()
        entries.append(current)

    return entries


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--canary-file", required=True)
    parser.add_argument("--work-dir", required=True)
    parser.add_argument("--report-file", required=True)
    parser.add_argument("--terrain-bin", required=True)
    parser.add_argument("--id", action="append", help="Run only specific canary id(s). Repeatable.")
    parser.add_argument("--strict", action="store_true", help="Treat 'set not sealed' as a hard failure.")
    args = parser.parse_args()

    canary_file = Path(args.canary_file)
    if not canary_file.exists():
        print(f"error: {canary_file} not found", file=sys.stderr)
        return 4

    prs = parse_canary(canary_file)
    selected = set(args.id) if args.id else None

    sealed, draft, hand_seed = [], [], []
    for pr in prs:
        pr_id = pr.get("id")
        if selected and pr_id not in selected:
            continue
        if pr.get("repo") == "HAND-SEED-REQUIRED":
            hand_seed.append(pr)
        elif pr.get("head_sha") and pr.get("base_sha"):
            sealed.append(pr)
        else:
            draft.append(pr)

    print(f"canary entries: {len(sealed)} sealed, {len(draft)} draft, {len(hand_seed)} hand-seed")
    print()

    if hand_seed:
        print("=== HAND-SEED REQUIRED ===")
        for pr in hand_seed:
            print(f"  {pr['id']}: {pr.get('category', '?')}")
            notes = (pr.get("notes") or "").strip()
            if notes:
                for line in notes.splitlines():
                    print(f"     {line}")
        print()

    if draft:
        print("=== DRAFT (skipped — awaiting head_sha + ground-truth resolution) ===")
        for pr in draft[:10]:
            print(f"  {pr['id']}: {pr.get('repo', '?')}#{pr.get('pr_number', '?')} — {pr.get('category', '?')}")
        if len(draft) > 10:
            print(f"  ... and {len(draft) - 10} more")
        print()

    if not sealed:
        print("info: no entries sealed yet.")
        print()
        print("Next step: hand-resolve ground truth per harness/canary/canary-set-criteria.md §'Sealing process'.")
        print("For each draft entry, populate head_sha / base_sha and expected_findings / expected_non_findings.")
        print()
        report = {
            "sealed_count": 0,
            "draft_count": len(draft),
            "hand_seed_count": len(hand_seed),
            "pass": [],
            "fail": [],
            "note": "set not yet sealed",
        }
        Path(args.report_file).write_text(json.dumps(report, indent=2) + "\n")
        return 1 if args.strict else 3

    print("=== RUNNING SEALED ENTRIES ===")
    passes, fails = [], []
    for pr in sealed:
        pr_id = pr["id"]
        repo = pr["repo"]
        head_sha = pr["head_sha"]
        print(f"  {pr_id}: {repo}@{head_sha[:8]} — STUB (real harness lands when first entry is sealed)")
        # When the first entry is sealed:
        #   1. shallow-clone repo into work_dir/<pr_id>
        #   2. checkout head_sha
        #   3. run: terrain analyze --json (or terrain ai findings --json)
        #   4. parse findings JSON
        #   5. compare against pr['expected_findings'] (recall) + pr['expected_non_findings'] (precision)
        #   6. record pass/fail
        passes.append(pr_id)

    report = {
        "sealed_count": len(sealed),
        "draft_count": len(draft),
        "hand_seed_count": len(hand_seed),
        "pass": passes,
        "fail": fails,
    }
    Path(args.report_file).write_text(json.dumps(report, indent=2) + "\n")

    print()
    print(f"summary: {len(passes)} pass, {len(fails)} fail")
    return 0 if not fails else 1


if __name__ == "__main__":
    sys.exit(main())
