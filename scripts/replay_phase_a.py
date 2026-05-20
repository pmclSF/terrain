#!/usr/bin/env python3
"""
Phase A offline replay — A.1, A.2, A.3, A.4, A.6 in one pass.

Each experiment estimates the TP/FP delta from applying a proposed filter,
using existing Claude verdicts and rationale text as heuristic ground truth.

Conservative: we use Claude's _verdict.reason to decide whether a row's
file *would* pass the new filter. Where the verdict reason is ambiguous,
the row is marked "uncertain" and shown separately so we don't over-claim.

Outputs: tier-4/phase-a-results.json + console summary table.
"""

from __future__ import annotations
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path


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


def reason_of(r: dict) -> str:
    v = r.get("_verdict") or {}
    return (v.get("reason") or "").lower()


def verdict_of(r: dict) -> str:
    return (r.get("_verdict") or {}).get("verdict", "?")


def for_detector(rows: list[dict], rule_id: str) -> list[dict]:
    return [r for r in rows if r.get("rule_id") == rule_id]


def precision(tp: int, fp: int) -> float:
    return tp / max(1, tp + fp) * 100


# --- A.1: migrationBlocker enzyme-import gate -------------------------------

def experiment_a1(rows: list[dict]) -> dict:
    mine = for_detector(rows, "migrationBlocker")
    # Heuristic: a file would PASS the enzyme-import gate iff the verdict
    # rationale gives positive evidence of enzyme presence.
    keep_positive = re.compile(
        r"\b(uses enzyme|enzyme is (imported|used)|imports? enzyme|"
        r"enzyme-adapter|from ['\"]enzyme['\"]|"
        r"enzyme\.\w+|shallow\(|mount\(.*enzyme)", re.IGNORECASE)
    drop_negative = re.compile(
        r"\b(no enzyme|without enzyme|doesn't use enzyme|"
        r"not using enzyme|uses (vitest|jest|rtl|playwright|testing-library|"
        r"react testing library)|modern|no longer uses enzyme)", re.IGNORECASE)

    tp_before = sum(1 for r in mine if verdict_of(r) == "TP")
    fp_before = sum(1 for r in mine if verdict_of(r) == "FP")
    kept_tp = kept_fp = 0
    dropped_tp = dropped_fp = 0
    ambiguous = 0
    for r in mine:
        reason = reason_of(r)
        v = verdict_of(r)
        if v not in ("TP", "FP"):
            continue
        # Positive evidence beats negative (more specific)
        if keep_positive.search(reason):
            if v == "TP":
                kept_tp += 1
            else:
                kept_fp += 1
        elif drop_negative.search(reason):
            if v == "TP":
                dropped_tp += 1
            else:
                dropped_fp += 1
        else:
            ambiguous += 1
            # Conservative: assume ambiguous would still fire (kept)
            if v == "TP":
                kept_tp += 1
            else:
                kept_fp += 1
    return {
        "name": "A.1 migrationBlocker enzyme-import gate",
        "detector": "migrationBlocker",
        "before": {"TP": tp_before, "FP": fp_before,
                   "prec": precision(tp_before, fp_before)},
        "after": {"TP": kept_tp, "FP": kept_fp,
                  "prec": precision(kept_tp, kept_fp)},
        "filter_dropped": {"TP": dropped_tp, "FP": dropped_fp},
        "ambiguous": ambiguous,
        "verdict": ("Detector still produces near-zero findings — capability "
                    "preserved as observability; promotion to gate would need "
                    "a corpus with live enzyme migrations."
                    if kept_tp == 0 else "Gate has measurable signal"),
    }


# --- A.2: weakAssertion regex-floor expansion -------------------------------

def experiment_a2(rows: list[dict]) -> dict:
    """If the regex floor recognized xUnit, mock-API, library-helper, fluent
    families, how many FPs would flip to TN?"""
    mine = for_detector(rows, "weakAssertion")
    family_caught = re.compile(
        r"\b(self\.assert\w+|assertequal|assertraises|asserttrue|"
        r"assertfalse|assertin|assertnotin|assertisinstance|"
        r"np\.testing\.assert|torch\.testing\.assert|"
        r"pd\.testing\.assert|tvm\.testing\.assert|chex\.assert|"
        r"assert_called|tohavebeencalled|mockito\.verify|"
        r"expect\([^)]*\)\.(to|tobeequal|tomatch)|"
        r"andexpect|onview\(|hamcrest|unittest|"
        r"pytest\.raises|pytest\.warns|"
        r"detector miscounted|missed (the )?assertion|"
        r"contains? (.*)?assert\w+|"
        r"contains (assertequal|assertraises|asserttrue))",
        re.IGNORECASE)
    tp_before = sum(1 for r in mine if verdict_of(r) == "TP")
    fp_before = sum(1 for r in mine if verdict_of(r) == "FP")
    kept_tp = kept_fp = 0
    dropped_tp = dropped_fp = 0
    for r in mine:
        reason = reason_of(r)
        v = verdict_of(r)
        if v not in ("TP", "FP"):
            continue
        if family_caught.search(reason):
            # Regex floor would recognize this assertion family -> detector
            # would see it -> would NOT fire -> filter drops
            if v == "TP":
                dropped_tp += 1
            else:
                dropped_fp += 1
        else:
            if v == "TP":
                kept_tp += 1
            else:
                kept_fp += 1
    return {
        "name": "A.2 weakAssertion regex-floor expansion (~30 tokens)",
        "detector": "weakAssertion",
        "before": {"TP": tp_before, "FP": fp_before,
                   "prec": precision(tp_before, fp_before)},
        "after": {"TP": kept_tp, "FP": kept_fp,
                  "prec": precision(kept_tp, kept_fp)},
        "filter_dropped": {"TP": dropped_tp, "FP": dropped_fp},
        "verdict": (f"Regex floor catches ~{dropped_fp/max(1,fp_before)*100:.0f}% "
                    f"of FPs at cost of ~{dropped_tp/max(1,tp_before)*100:.0f}% "
                    "of TPs — assesses regex-floor ceiling vs full A1 AST oracle"),
    }


# --- A.3: testsOnlyMocks path-role gate -------------------------------------

def experiment_a3(rows: list[dict]) -> dict:
    """If we gate testsOnlyMocks on `path is a real test file (not fixture/
    conftest/helper)`, how many FPs drop?"""
    mine = for_detector(rows, "testsOnlyMocks")
    # File-path heuristic for "not a test"
    nontest_path = re.compile(
        r"(conftest\.py|/fixtures?/|/helpers?/|/utils/|"
        r"_helpers?\.py|setup\.py|/vendored?/|"
        r"node_modules/|/third_party/|/_vendor/|/examples/|/demo)",
        re.IGNORECASE)
    # Verdict-reason heuristic
    nontest_reason = re.compile(
        r"\b(not a test|isn't a test|conftest|fixture|helper|"
        r"setup file|example|demo|vendored)\b", re.IGNORECASE)

    tp_before = sum(1 for r in mine if verdict_of(r) == "TP")
    fp_before = sum(1 for r in mine if verdict_of(r) == "FP")
    kept_tp = kept_fp = 0
    dropped_tp = dropped_fp = 0
    for r in mine:
        v = verdict_of(r)
        if v not in ("TP", "FP"):
            continue
        fp_path = r.get("file", "")
        reason = reason_of(r)
        is_nontest = bool(nontest_path.search(fp_path)
                          or nontest_reason.search(reason))
        if is_nontest:
            if v == "TP":
                dropped_tp += 1
            else:
                dropped_fp += 1
        else:
            if v == "TP":
                kept_tp += 1
            else:
                kept_fp += 1
    return {
        "name": "A.3 testsOnlyMocks Role==RoleTest gate",
        "detector": "testsOnlyMocks",
        "before": {"TP": tp_before, "FP": fp_before,
                   "prec": precision(tp_before, fp_before)},
        "after": {"TP": kept_tp, "FP": kept_fp,
                  "prec": precision(kept_tp, kept_fp)},
        "filter_dropped": {"TP": dropped_tp, "FP": dropped_fp},
        "verdict": ("Bounds the value of a simple path-role gate vs full "
                    "A2 import-graph machinery"),
    }


# --- A.4: uncoveredAISurface sub-lane split + LLM-proximity -----------------

def experiment_a4(rows: list[dict]) -> dict:
    """Three-lane split per the moat preservation plan:
       - aiPrompt lane: keep all (its precision is highest)
       - aiModel lane: require LLM-call-site OR named-prompt-shape (drop Zod/
         decorator/synthesized stems)
       - aiDataset lane: defer (n too small to filter)
    """
    mine = for_detector(rows, "uncoveredAISurface")

    # Sub-lane detection (from evidence/title text + symbol shape)
    def lane(r: dict) -> str:
        ev = (r.get("evidence", "") + " " + r.get("title", "")).lower()
        if "ai prompt" in ev or "prompt template" in ev:
            return "prompt"
        if "ai model" in ev or "model" in ev:
            return "model"
        if "ai dataset" in ev or "dataset" in ev:
            return "dataset"
        return "other"

    # Filter for aiModel lane: keep only if symbol looks like a real
    # LLM call site / named constant, not a Zod / decorator / synthesized.
    bad_model_shape = re.compile(
        r"(^zod_|.*schema$|.*props$|.*config$|.*params$|"
        r"_l\d+$|.*type$|.*request$|.*response$)", re.IGNORECASE)
    good_model_evidence = re.compile(
        r"\b(openai|anthropic|chatcompletion|chat\.completion|"
        r"messages\.create|langchain|llm\.invoke|client\.invoke|"
        r"generatetext|.generate\()", re.IGNORECASE)
    bad_model_reason = re.compile(
        r"\b(zod|pydantic|schema|decorator|type alias|"
        r"validation model|data model|not an? (ai|llm) (model|surface)|"
        r"http request|configuration|i18n|locale)", re.IGNORECASE)

    by_lane = defaultdict(lambda: {"TP": 0, "FP": 0,
                                   "kept_TP": 0, "kept_FP": 0,
                                   "dropped_TP": 0, "dropped_FP": 0})
    for r in mine:
        v = verdict_of(r)
        if v not in ("TP", "FP"):
            continue
        ln = lane(r)
        by_lane[ln][v] += 1
        symbol = (r.get("symbol", "") or "").lower()
        reason = reason_of(r)

        if ln == "prompt":
            # Keep all — strongest lane
            keep = True
        elif ln == "model":
            # Drop if symbol shape is bad OR Claude says it isn't an AI surface
            keep = not (bad_model_shape.search(symbol)
                        or bad_model_reason.search(reason))
            # Also keep if there's positive LLM-call evidence
            if not keep and good_model_evidence.search(reason):
                keep = True
        elif ln == "dataset":
            keep = True  # too small to filter
        else:
            keep = True
        if keep:
            by_lane[ln]["kept_" + v] += 1
        else:
            by_lane[ln]["dropped_" + v] += 1

    tp_before = sum(by_lane[ln]["TP"] for ln in by_lane)
    fp_before = sum(by_lane[ln]["FP"] for ln in by_lane)
    kept_tp = sum(by_lane[ln]["kept_TP"] for ln in by_lane)
    kept_fp = sum(by_lane[ln]["kept_FP"] for ln in by_lane)
    dropped_tp = sum(by_lane[ln]["dropped_TP"] for ln in by_lane)
    dropped_fp = sum(by_lane[ln]["dropped_FP"] for ln in by_lane)

    return {
        "name": ("A.4 uncoveredAISurface 3-lane split + aiModel "
                 "LLM-call-site gate"),
        "detector": "uncoveredAISurface",
        "before": {"TP": tp_before, "FP": fp_before,
                   "prec": precision(tp_before, fp_before)},
        "after": {"TP": kept_tp, "FP": kept_fp,
                  "prec": precision(kept_tp, kept_fp)},
        "filter_dropped": {"TP": dropped_tp, "FP": dropped_fp},
        "by_lane": {ln: {
            "TP": c["TP"], "FP": c["FP"],
            "prec_before": precision(c["TP"], c["FP"]),
            "kept_TP": c["kept_TP"], "kept_FP": c["kept_FP"],
            "prec_after": precision(c["kept_TP"], c["kept_FP"]),
        } for ln, c in by_lane.items()},
        "verdict": ("Bounds A4 moat work — quantifies sub-lane split + "
                    "aiModel LLM-call gate impact"),
    }


# --- A.6: Co-firing dedup ---------------------------------------------------

def experiment_a6(rows: list[dict]) -> dict:
    """When N detectors fire on the same (repo, file), emit only the
    highest-precision detector's finding. Measures volume reduction and net
    aggregate precision change."""

    # Per-detector precision (use merged n=250 as the ranking)
    det_stats = defaultdict(lambda: {"TP": 0, "FP": 0})
    for r in rows:
        v = verdict_of(r)
        if v in ("TP", "FP"):
            det_stats[r.get("rule_id", "?")][v] += 1
    det_prec = {d: precision(c["TP"], c["FP"])
                for d, c in det_stats.items()}

    # Group rows by (repo, file)
    by_loc = defaultdict(list)
    for r in rows:
        v = verdict_of(r)
        if v not in ("TP", "FP"):
            continue
        by_loc[(r.get("repo", ""), r.get("file", ""))].append(r)

    # Before
    tp_before = sum(1 for r in rows if verdict_of(r) == "TP")
    fp_before = sum(1 for r in rows if verdict_of(r) == "FP")

    # After dedup: per location, keep only the row from the highest-precision
    # detector. (In practice we'd keep the one with the most actionable fix —
    # this is a precision-ceiling estimate.)
    kept_tp = kept_fp = 0
    suppressed_findings = 0
    multi_fire_locations = 0
    for loc, rs in by_loc.items():
        if len(rs) > 1:
            multi_fire_locations += 1
        # Sort rows at this location by detector precision (desc)
        rs_sorted = sorted(
            rs, key=lambda r: -det_prec.get(r.get("rule_id"), 0))
        winner = rs_sorted[0]
        suppressed_findings += len(rs) - 1
        if verdict_of(winner) == "TP":
            kept_tp += 1
        else:
            kept_fp += 1

    return {
        "name": "A.6 Co-firing dedup (highest-precision detector wins per file)",
        "detector": "(all)",
        "before": {"TP": tp_before, "FP": fp_before,
                   "prec": precision(tp_before, fp_before)},
        "after": {"TP": kept_tp, "FP": kept_fp,
                  "prec": precision(kept_tp, kept_fp)},
        "multi_fire_locations": multi_fire_locations,
        "suppressed_findings": suppressed_findings,
        "total_locations": len(by_loc),
        "verdict": (f"{multi_fire_locations}/{len(by_loc)} "
                    f"({multi_fire_locations/max(1,len(by_loc))*100:.1f}%) "
                    "locations have multi-detector firings; dedup saves "
                    f"{suppressed_findings} findings"),
    }


# --- Main -------------------------------------------------------------------

def main():
    rows = load_merged()
    print(f"loaded {len(rows)} deduped rows", file=sys.stderr)

    results = [
        experiment_a1(rows),
        experiment_a2(rows),
        experiment_a3(rows),
        experiment_a4(rows),
        experiment_a6(rows),
    ]

    print()
    print("=" * 100)
    print("Phase A — Offline replay results (n=250 merged corpus)")
    print("=" * 100)
    print()
    for r in results:
        b = r["before"]
        a = r["after"]
        d = r.get("filter_dropped", {"TP": 0, "FP": 0})
        print(f"### {r['name']}")
        print(f"  Detector: {r['detector']}")
        print(f"  Before:   TP={b['TP']:4d}  FP={b['FP']:4d}  "
              f"prec={b['prec']:5.1f}%")
        print(f"  After:    TP={a['TP']:4d}  FP={a['FP']:4d}  "
              f"prec={a['prec']:5.1f}%  "
              f"({a['prec'] - b['prec']:+.1f}pp)")
        if d:
            print(f"  Dropped:  TP={d['TP']:4d} (recall cost)  "
                  f"FP={d['FP']:4d} (precision lift)")
        if "by_lane" in r:
            print(f"  By sub-lane:")
            for ln, c in r["by_lane"].items():
                print(f"    {ln:8s}: "
                      f"before TP={c['TP']:3d} FP={c['FP']:3d} "
                      f"prec={c['prec_before']:5.1f}%  ->  "
                      f"after TP={c['kept_TP']:3d} FP={c['kept_FP']:3d} "
                      f"prec={c['prec_after']:5.1f}%")
        if "multi_fire_locations" in r:
            print(f"  Multi-fire locations: "
                  f"{r['multi_fire_locations']}/{r['total_locations']}")
            print(f"  Suppressed findings:  {r['suppressed_findings']}")
        print(f"  Verdict: {r['verdict']}")
        print()

    out = Path("tier-4/phase-a-results.json")
    with out.open("w") as f:
        json.dump(results, f, indent=2)
    print(f"Results JSON: {out}")


if __name__ == "__main__":
    main()
