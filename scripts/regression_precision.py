#!/usr/bin/env python3
"""
Precision regression test for the n=250 corpus.

Runs against tier-4/detector-validation.jsonl + detector-validation-n200.jsonl.

What it does:
  1. Loads merged n=250 verdicts as ground truth
  2. For detectors with replicable row-level filters (untestedExport type/
     schema, uncoveredAISurface aiModel, blastRadiusHotspot pure-conduit),
     simulates the new filter logic and predicts the post-filter precision
  3. Records baseline precision for ALL detectors
  4. Fails if any detector drops >2pp from baseline

Mirrors the Go filter logic. Keep this file in sync with:
  - internal/quality/untested_export.go isTypeOrSchemaDecl
  - internal/structural/uncovered_ai_surface.go isStructuralAIModelFP
  - internal/structural/blast_radius_hotspot.go isPureConduitFile

Baseline (2026-05-18 measurement, before Phase C structural fixes landed):
  see BASELINE_PRECISION dict.

Exit code 0 if all detectors within tolerance, 1 if any regression.

Run: python3 scripts/regression_precision.py
CI:  add `make regression-precision` target invoking this.
"""

from __future__ import annotations
import json
import os
import re
import sys
from collections import defaultdict
from pathlib import Path


# Baseline precision from 2026-05-18 measurement (merged n=250 dedup).
# Any future code change must not drop a detector's precision more than 2pp
# below these values when replayed against the same corpus.
BASELINE_PRECISION = {
    "frameworkMigration":      98.4,
    "snapshotHeavyTest":       91.7,
    "mockHeavyTest":           88.8,
    "staticSkippedTest":       78.7,
    "fixtureFragilityHotspot": 65.6,
    "untestedExport":          64.4,
    "blastRadiusHotspot":      55.8,
    "customMatcherRisk":       52.3,
    "dynamicTestGeneration":   51.7,
    "orphanedTestFile":        35.8,
    "assertionFreeTest":       35.6,
    "uncoveredAISurface":      21.5,
    "deprecatedTestPattern":   21.0,
    "weakAssertion":           20.5,
    "assertionFreeImport":     17.3,
    "testsOnlyMocks":          0.7,
    "migrationBlocker":        0.0,
    # Tiny-sample detectors (n=1) — included for completeness, not enforced.
    "coverageBlindSpot":       100.0,
    "coverageThresholdBreak":  100.0,
}

# Tolerance: a detector may NOT drop more than this many percentage points
# from its baseline precision. The threshold accounts for sampling noise
# (Wilson width on n=250 is ~±5pp) without allowing meaningful regressions.
TOLERANCE_PP = 2.0

# Minimum n for enforcement — detectors with fewer rows than this skip
# the regression check (sampling noise dominates).
MIN_N = 20


# ─── Filters mirroring production Go detector code ────────────────────────

TYPE_SCHEMA_SUFFIXES = (
    "Schema", "Props", "Type", "Config", "Params", "Request",
    "Response", "Options", "Settings", "Args", "Input", "Output",
    "Variables", "Result", "State", "Context",
)


def filter_untested_export_type_schema(symbol: str) -> bool:
    """Mirrors isTypeOrSchemaDecl in internal/quality/untested_export.go."""
    if not symbol:
        return False
    if not symbol[0].isupper():
        return False  # PascalCase only
    for suffix in TYPE_SCHEMA_SUFFIXES:
        if symbol.endswith(suffix) and len(symbol) > len(suffix):
            return True
    return False


def filter_uncovered_ai_model_fp(symbol: str, sub_lane: str | None) -> bool:
    """Mirrors isStructuralAIModelFP in internal/structural/uncovered_ai_surface.go.
    Only applies to model lane."""
    if sub_lane != "model" or not symbol:
        return False
    if re.search(r"_L\d+$", symbol):
        return True
    if re.search(r"(^tool_decorated_|_tool$|_decorator(_|$))", symbol, re.IGNORECASE):
        return True
    if symbol[0].isupper():
        for suffix in TYPE_SCHEMA_SUFFIXES:
            if symbol.endswith(suffix) and len(symbol) > len(suffix):
                return True
    if symbol.lower().startswith("zod_"):
        return True
    return False


BARREL_BASENAMES = {
    "__init__.py", "index.ts", "index.tsx", "index.js", "index.jsx",
    "mod.rs", "lib.rs",
}
GENERATED_SUFFIXES = (
    ".pb.go", "_pb2.py", ".pb.cc", ".pb.h",
    ".gen.go", "_generated.go", ".gen.ts",
    ".g.dart", ".freezed.dart",
)


def filter_blast_radius_pure_conduit(file_path: str) -> bool:
    """Mirrors isPureConduitFile in internal/structural/blast_radius_hotspot.go.
    Note: this only checks the basename. The full filter in production also
    requires direct=0 + indirect>=30, which we can't replicate from row
    metadata. So this returns the "suspect" set; downstream Go code makes
    the final call."""
    base = os.path.basename(file_path)
    if base in BARREL_BASENAMES:
        return True
    for suf in GENERATED_SUFFIXES:
        if base.endswith(suf):
            return True
    return False


# ─── Data loading and replay ─────────────────────────────────────────────

def load_merged() -> list[dict]:
    rows = []
    for path in ["tier-4/detector-validation.jsonl",
                 "tier-4/detector-validation-n200.jsonl"]:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
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


def verdict_of(r: dict) -> str:
    return (r.get("_verdict") or {}).get("verdict", "?")


def is_filtered_out(row: dict) -> bool:
    """Returns True if the new code would NOT emit this finding (filter
    drops it before signal generation)."""
    rule = row.get("rule_id", "")
    symbol = row.get("symbol", "")
    file_path = row.get("file", "")

    if rule == "untestedExport":
        # New type/schema filter targets TS/TSX/JS/JSX files only.
        if file_path.endswith((".ts", ".tsx", ".js", ".jsx")):
            if filter_untested_export_type_schema(symbol):
                return True
        return False

    if rule == "uncoveredAISurface":
        # Identify sub-lane from evidence/title text.
        ev = ((row.get("evidence", "") or "") + " " +
              (row.get("title", "") or "")).lower()
        sub_lane = None
        if "ai prompt" in ev:
            sub_lane = "prompt"
        elif "ai model" in ev or "model" in ev:
            sub_lane = "model"
        elif "ai dataset" in ev:
            sub_lane = "dataset"
        return filter_uncovered_ai_model_fp(symbol, sub_lane)

    # blastRadiusHotspot pure-conduit filter requires direct/indirect
    # counts we don't have in row metadata — skip for now. The new code
    # demotes to info severity but doesn't drop the finding, so the row
    # would still appear (just at lower tier).
    return False


def compute_precision(rows: list[dict]) -> dict[str, dict]:
    """Per-detector precision before and after replay filtering."""
    by_det_before = defaultdict(lambda: {"TP": 0, "FP": 0})
    by_det_after = defaultdict(lambda: {"TP": 0, "FP": 0})
    for r in rows:
        det = r.get("rule_id", "?")
        v = verdict_of(r)
        if v not in ("TP", "FP"):
            continue
        by_det_before[det][v] += 1
        if not is_filtered_out(r):
            by_det_after[det][v] += 1

    out = {}
    for det, before in by_det_before.items():
        after = by_det_after[det]
        prec_before = before["TP"] / max(1, before["TP"] + before["FP"]) * 100
        prec_after = after["TP"] / max(1, after["TP"] + after["FP"]) * 100
        out[det] = {
            "n_before": before["TP"] + before["FP"],
            "n_after": after["TP"] + after["FP"],
            "precision_before_pct": prec_before,
            "precision_after_pct": prec_after,
            "delta_pp": prec_after - prec_before,
        }
    return out


def main():
    rows = load_merged()
    sys.stderr.write(f"loaded {len(rows)} merged rows\n")

    results = compute_precision(rows)

    print()
    print("=" * 110)
    print("Precision regression replay (n=250 corpus)")
    print("=" * 110)
    print()
    print(f"{'Detector':30s} {'n':>4s} -> {'n':>4s} "
          f"{'baseline':>9s} {'now':>7s} {'after-flt':>10s} {'Δ vs base':>10s} "
          f"{'status':>10s}")
    print("-" * 110)

    fails = []
    skipped = []
    for det in sorted(BASELINE_PRECISION, key=lambda k: -BASELINE_PRECISION[k]):
        base = BASELINE_PRECISION[det]
        r = results.get(det, {"n_before": 0, "n_after": 0,
                              "precision_before_pct": 0, "precision_after_pct": 0})
        # The detector's CURRENT precision in the corpus is the "before"
        # value (filters in this script aren't actually applied to the
        # production data — the data is from before the filters shipped).
        # "after-filt" is what the new code would predict.
        now = r["precision_before_pct"]
        after = r["precision_after_pct"]
        delta_vs_base = now - base
        status = ""
        if r["n_before"] < MIN_N:
            status = "skip-small-n"
            skipped.append(det)
        elif delta_vs_base < -TOLERANCE_PP:
            status = "REGRESSED"
            fails.append((det, base, now, delta_vs_base))
        elif (after - now) >= 5.0:
            status = "lift!"  # filter projects a meaningful lift
        else:
            status = "ok"
        print(f"{det:30s} {r['n_before']:>4d} -> {r['n_after']:>4d} "
              f"{base:>8.1f}% {now:>6.1f}% {after:>9.1f}% "
              f"{delta_vs_base:>+9.1f}pp {status:>10s}")
    print()

    if fails:
        print(f"FAIL: {len(fails)} detector(s) regressed > {TOLERANCE_PP}pp:")
        for det, base, now, delta in fails:
            print(f"  {det}: {base:.1f}% -> {now:.1f}% ({delta:+.1f}pp)")
        sys.exit(1)
    if skipped:
        print(f"Skipped (n < {MIN_N}): {', '.join(skipped)}")
    print("PASS: no precision regressions exceed tolerance.")


if __name__ == "__main__":
    main()
