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
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path


def parse_canary(path):
    """Parse harness/canary/canary-set.yaml using only stdlib. Returns list of
    dicts with id / repo / pr_number / head_sha / base_sha / category /
    notes (raw text) / expected_findings (list of dicts) /
    expected_non_findings (list of dicts)."""
    entries = []
    current = None
    current_key_indent = None
    notes_lines = None
    notes_indent = None
    # Nested-list state for expected_findings / expected_non_findings.
    list_key = None
    list_items = None
    list_indent = None
    list_item_indent = None
    current_item = None

    line_re_id = re.compile(r"^(\s*)- id:\s*(canary-\S+)\s*$")
    line_re_kv = re.compile(r"^(\s+)([\w_]+):\s*(.*)$")
    line_re_item = re.compile(r"^(\s*)-\s+([\w_]+):\s*(.*)$")

    def flush_list():
        nonlocal list_key, list_items, list_indent, list_item_indent, current_item
        if list_key is None or current is None:
            return
        if current_item is not None:
            list_items.append(current_item)
            current_item = None
        current[list_key] = list_items or []
        list_key = None
        list_items = None
        list_indent = None
        list_item_indent = None

    def flush_notes():
        nonlocal notes_lines, notes_indent
        if notes_lines is None or current is None:
            return
        key_set_to = current.pop("_active_notes_key", "notes")
        current[key_set_to] = "\n".join(notes_lines).strip()
        notes_lines = None
        notes_indent = None

    for raw in path.read_text().splitlines():
        indent = len(raw) - len(raw.lstrip(" "))
        stripped = raw.rstrip()

        if notes_lines is not None:
            if not stripped:
                notes_lines.append("")
                continue
            if indent > notes_indent:
                notes_lines.append(raw[notes_indent + 2 :])
                continue
            flush_notes()

        if list_key is not None:
            if not stripped:
                continue
            if indent <= list_indent and not re.match(r"^\s*#", raw):
                flush_list()
            else:
                m_item = line_re_item.match(raw)
                if m_item and len(m_item.group(1)) == list_indent + 2:
                    if current_item is not None:
                        list_items.append(current_item)
                    current_item = {m_item.group(2): m_item.group(3).strip()}
                    list_item_indent = len(m_item.group(1)) + 2
                    continue
                m_kv = line_re_kv.match(raw)
                if m_kv and current_item is not None and indent == list_item_indent:
                    current_item[m_kv.group(2)] = m_kv.group(3).strip()
                continue

        m = line_re_id.match(raw)
        if m:
            flush_list()
            flush_notes()
            if current is not None:
                entries.append(current)
            current = {"id": m.group(2)}
            current_key_indent = len(m.group(1)) + 2
            continue

        if current is None:
            continue

        m = line_re_kv.match(raw)
        if not m:
            continue
        kv_indent = len(m.group(1))
        if kv_indent != current_key_indent:
            continue
        key = m.group(2)
        value = m.group(3).strip()

        if key in ("notes", "why_canary"):
            notes_lines = []
            notes_indent = kv_indent
            current["_active_notes_key"] = key
            continue

        if key in ("expected_findings", "expected_non_findings"):
            list_key = key
            list_items = []
            list_indent = kv_indent
            list_item_indent = None
            current_item = None
            continue

        if key in {"id", "repo", "pr_url", "merged_at", "category", "head_sha", "base_sha"}:
            current[key] = value if value not in ("null", "") else None
        elif key == "pr_number":
            try:
                current[key] = int(value) if value not in ("null", "") else None
            except ValueError:
                current[key] = value

    if current is not None:
        flush_list()
        flush_notes()
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
    work_root = Path(args.work_dir)
    work_root.mkdir(parents=True, exist_ok=True)
    passes, fails, per_pr = [], [], []
    for pr in sealed:
        pr_id = pr["id"]
        result = run_one(pr, work_root, args.terrain_bin)
        per_pr.append(result)
        if result["status"] == "pass":
            passes.append(pr_id)
            print(f"  {pr_id}: PASS ({len(result.get('findings_by_detector', {}))} detector(s) fired)")
        else:
            fails.append(pr_id)
            print(f"  {pr_id}: FAIL — {result.get('reason', '?')}")

    report = {
        "sealed_count": len(sealed),
        "draft_count": len(draft),
        "hand_seed_count": len(hand_seed),
        "pass": passes,
        "fail": fails,
        "details": per_pr,
    }
    Path(args.report_file).write_text(json.dumps(report, indent=2) + "\n")

    print()
    print(f"summary: {len(passes)} pass, {len(fails)} fail")
    # UFPP aggregate across sealed runs.
    ufpps = [d["ufpp"] for d in per_pr if d.get("ufpp") is not None]
    if ufpps:
        ufpps_sorted = sorted(ufpps)
        median = ufpps_sorted[len(ufpps_sorted) // 2]
        print(f"median UFPP across {len(ufpps)} sealed entries: {median:.2f}")
    return 0 if not fails else 1


def run_one(pr, work_root, terrain_bin):
    """Run terrain against one sealed canary PR. Returns a dict with status,
    findings_by_detector, ufpp, and recall/precision violations."""
    pr_id = pr["id"]
    repo = pr["repo"]  # "github.com/owner/repo"
    head_sha = pr["head_sha"]
    if not repo.startswith("github.com/"):
        return {"id": pr_id, "status": "fail", "reason": f"unsupported repo host: {repo}"}
    owner_name = repo[len("github.com/"):]

    pr_dir = work_root / pr_id
    if pr_dir.exists():
        shutil.rmtree(pr_dir)
    pr_dir.mkdir(parents=True)

    # Shallow-fetch the head SHA. `git init` + `git fetch --depth=1 <sha>`
    # is faster than full clone when we only need one commit.
    clone_url = f"https://github.com/{owner_name}.git"
    try:
        subprocess.check_call(["git", "init", "-q", str(pr_dir)], timeout=30)
        subprocess.check_call(
            ["git", "-C", str(pr_dir), "remote", "add", "origin", clone_url],
            timeout=30,
        )
        subprocess.check_call(
            ["git", "-C", str(pr_dir), "fetch", "--depth=1", "origin", head_sha],
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=180,
        )
        subprocess.check_call(
            ["git", "-C", str(pr_dir), "checkout", "-q", head_sha],
            timeout=30,
        )
    except subprocess.CalledProcessError as e:
        err = e.stderr.decode() if hasattr(e, "stderr") and e.stderr else str(e)
        return {"id": pr_id, "status": "fail", "reason": f"git fetch: {err[:120]}"}
    except subprocess.TimeoutExpired:
        return {"id": pr_id, "status": "fail", "reason": "git fetch timeout"}

    # Run terrain analyze. The exit code is non-zero when findings exist
    # at a high enough severity to fail the gate; for canary purposes we
    # capture stdout regardless.
    try:
        out = subprocess.run(
            [terrain_bin, "analyze", "--root", str(pr_dir), "--json"],
            capture_output=True, timeout=600,
        )
        report = json.loads(out.stdout) if out.stdout else {}
    except subprocess.TimeoutExpired:
        return {"id": pr_id, "status": "fail", "reason": "terrain analyze timeout"}
    except json.JSONDecodeError as e:
        return {"id": pr_id, "status": "fail", "reason": f"terrain JSON parse: {e}"}

    # Extract per-detector firing counts. The analyze --json output's
    # signalSummary.byType is the canonical per-rule_id breakdown.
    summary = report.get("signalSummary", {})
    total = summary.get("total", 0)
    by_det = summary.get("byType", {}) or {}

    # Compare against expected_findings (recall) and expected_non_findings
    # (precision).
    recall_misses = []
    for expected in pr.get("expected_findings") or []:
        rule = expected.get("rule_id")
        if not rule:
            continue
        if by_det.get(rule, 0) == 0:
            recall_misses.append(rule)

    precision_violations = []
    for unexpected in pr.get("expected_non_findings") or []:
        rule = unexpected.get("rule_id")
        if rule == "ANY_AI_RULE":
            # Any AI-categorized firing is a violation.
            ai_total = sum(v for k, v in by_det.items() if "ai" in k.lower() or "prompt" in k.lower())
            if ai_total > 0:
                precision_violations.append(f"ANY_AI_RULE: {ai_total} fired")
        elif rule and by_det.get(rule, 0) > 0:
            precision_violations.append(f"{rule}: {by_det[rule]} fired")

    status = "pass"
    reasons = []
    if recall_misses:
        status = "fail"
        reasons.append(f"recall miss: {', '.join(recall_misses)}")
    if precision_violations:
        status = "fail"
        reasons.append(f"precision violation: {'; '.join(precision_violations)}")

    return {
        "id": pr_id,
        "status": status,
        "reason": " | ".join(reasons) if reasons else None,
        "findings_total": total,
        "findings_by_detector": by_det,
        "ufpp": float(total),
        "recall_misses": recall_misses,
        "precision_violations": precision_violations,
    }


if __name__ == "__main__":
    sys.exit(main())
