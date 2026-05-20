#!/usr/bin/env python3
"""
Aggregate Claude validation + (optional) OpenAI cross-check into
per-detector readiness cards.

Reads:
    tier-4/llm-rated-validation.jsonl     (required, Claude verdicts)
    tier-4/openai-crosscheck.jsonl        (optional, GPT-4o verdicts)

Writes:
    tier-4/detector-readiness-cards.md    (human-readable per-detector cards)
    tier-4/detector-readiness.jsonl       (machine-readable: one record per detector)

For each detector, the card shows:
  - sample size, TP/FP/UNCERTAIN counts
  - Claude-only precision + Wilson 95% CI
  - cross-confirmed precision (if OpenAI data present)
  - top 5 FP reasons (deduped by leading verb/noun)
  - recommended tier (gate / observability / drop)
"""

import argparse
import json
import math
import sys
from collections import defaultdict, Counter
from pathlib import Path


def wilson_ci(k: int, n: int, z: float = 1.96) -> tuple[float, float, float]:
    """Wilson score interval. Returns (point, low, high) all in [0,1]."""
    if n == 0:
        return (0.0, 0.0, 0.0)
    p = k / n
    denom = 1 + z * z / n
    centre = (p + z * z / (2 * n)) / denom
    half = z * math.sqrt((p * (1 - p) / n) + (z * z / (4 * n * n))) / denom
    return (p, max(0.0, centre - half), min(1.0, centre + half))


def tier_for_precision(precision: float, n: int, lo: float) -> str:
    """Recommend a tier based on precision and CI lower bound."""
    if n < 10:
        return "INSUFFICIENT_DATA"
    if lo >= 0.80:
        return "gate-tier"
    if precision >= 0.60:
        return "observability-tier"
    return "needs-rework"


def top_fp_reasons(records: list, k: int = 5) -> list[tuple[str, int]]:
    """Bucket FP reasons by first ~6 words. Returns top-k (bucket, count)."""
    buckets = Counter()
    for r in records:
        v = (r.get("_verdict") or {}).get("verdict")
        if v != "FP":
            continue
        reason = (r.get("_verdict") or {}).get("reason", "").strip()
        # Bucket by first significant phrase
        words = reason.split()
        bucket = " ".join(words[:6]).rstrip(",.;:")
        if len(bucket) > 80:
            bucket = bucket[:77] + "..."
        buckets[bucket] += 1
    return buckets.most_common(k)


def load_jsonl(path: Path) -> list:
    if not path.exists():
        return []
    out = []
    with path.open() as f:
        for line in f:
            try:
                out.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--validation", default="tier-4/llm-rated-validation.jsonl")
    ap.add_argument("--crosscheck", default="tier-4/openai-crosscheck.jsonl")
    ap.add_argument("--md-out", default="tier-4/detector-readiness-cards.md")
    ap.add_argument("--json-out", default="tier-4/detector-readiness.jsonl")
    args = ap.parse_args()

    validation = load_jsonl(Path(args.validation))
    crosscheck = load_jsonl(Path(args.crosscheck))
    if not validation:
        sys.exit(f"no validation records in {args.validation}")

    sys.stderr.write(f"[aggregate] {len(validation)} Claude records, "
                     f"{len(crosscheck)} cross-checked\n")

    # Index cross-check by record_id for fast lookup
    xc_by_id = {r.get("_record_id"): r for r in crosscheck if r.get("_record_id")}

    # Group validation records by detector (edge_type)
    by_detector = defaultdict(list)
    for r in validation:
        by_detector[r.get("edge_type", "(unknown)")].append(r)

    md_out = Path(args.md_out)
    json_out = Path(args.json_out)
    md_out.parent.mkdir(parents=True, exist_ok=True)

    cards_md = ["# Detector Readiness Cards",
                "",
                f"_Generated from {len(validation)} Claude-rated boundary records, "
                f"{len(crosscheck)} OpenAI cross-checks._",
                ""]
    summary_rows = []

    # Sort detectors by precision (descending) for nice ordering
    detector_stats = []
    for det, recs in by_detector.items():
        n = len(recs)
        tp = sum(1 for r in recs if (r.get("_verdict") or {}).get("verdict") == "TP")
        fp = sum(1 for r in recs if (r.get("_verdict") or {}).get("verdict") == "FP")
        unc = sum(1 for r in recs if (r.get("_verdict") or {}).get("verdict") == "UNCERTAIN")
        committed = tp + fp
        precision, lo, hi = wilson_ci(tp, committed)
        detector_stats.append((det, recs, n, tp, fp, unc, precision, lo, hi))

    detector_stats.sort(key=lambda x: -x[6])  # by precision desc

    for det, recs, n, tp, fp, unc, precision, lo, hi in detector_stats:
        committed = tp + fp
        tier = tier_for_precision(precision, committed, lo)

        # Cross-check stats — only for records that were sampled
        xc_recs = [xc_by_id[r["_record_id"]] for r in recs
                   if r.get("_record_id") in xc_by_id]
        cross_lines = []
        cross_summary = {}
        if xc_recs:
            xc_tp_tp = sum(1 for r in xc_recs
                           if (r.get("_verdict") or {}).get("verdict") == "TP"
                           and (r.get("_openai_verdict") or {}).get("verdict") == "TP")
            xc_fp_fp = sum(1 for r in xc_recs
                           if (r.get("_verdict") or {}).get("verdict") == "FP"
                           and (r.get("_openai_verdict") or {}).get("verdict") == "FP")
            xc_disagree = sum(1 for r in xc_recs
                              if not r.get("_agreement", False)
                              and (r.get("_verdict") or {}).get("verdict") != "UNCERTAIN"
                              and (r.get("_openai_verdict") or {}).get("verdict") != "UNCERTAIN")
            n_xc = len(xc_recs)
            both_agree_tp = xc_tp_tp
            consensus_p, consensus_lo, consensus_hi = wilson_ci(xc_tp_tp, xc_tp_tp + xc_fp_fp + xc_disagree)
            cross_lines = [
                f"- **Cross-check (n={n_xc})**: both models agree TP {xc_tp_tp}/{n_xc}, both FP {xc_fp_fp}/{n_xc}",
                f"- **Consensus precision** (both-TP / total committed): "
                f"{consensus_p:.0%} (Wilson 95% CI [{consensus_lo:.0%}, {consensus_hi:.0%}])",
            ]
            cross_summary = {
                "crosscheck_n": n_xc,
                "both_tp": xc_tp_tp,
                "both_fp": xc_fp_fp,
                "consensus_precision": round(consensus_p, 4),
                "consensus_ci_lo": round(consensus_lo, 4),
                "consensus_ci_hi": round(consensus_hi, 4),
            }

        # Top FP reasons
        fp_reasons = top_fp_reasons(recs, k=5)

        # Card markdown
        cards_md.append(f"## `{det}`")
        cards_md.append("")
        cards_md.append(f"- **Recommended tier**: **{tier.upper()}**")
        cards_md.append(f"- **Sample**: n={n} (TP={tp}, FP={fp}, UNCERTAIN={unc})")
        cards_md.append(
            f"- **Claude precision** (TP/(TP+FP)): "
            f"**{precision:.0%}** (Wilson 95% CI [{lo:.0%}, {hi:.0%}])"
        )
        if unc > 0 and n > 0:
            cards_md.append(
                f"- **UNCERTAIN rate**: {unc}/{n} = {100*unc/n:.0f}% "
                "(record alone insufficient for verdict)"
            )
        cards_md.extend(cross_lines)
        if fp_reasons:
            cards_md.append("- **Top FP failure modes** (top 5 by count):")
            for bucket, count in fp_reasons:
                cards_md.append(f"  - ({count}×) {bucket}")
        cards_md.append("")

        # JSON summary row
        summary = {
            "detector": det,
            "n_total": n,
            "n_tp": tp,
            "n_fp": fp,
            "n_uncertain": unc,
            "claude_precision": round(precision, 4),
            "claude_ci_lo": round(lo, 4),
            "claude_ci_hi": round(hi, 4),
            "uncertain_rate": round(unc / n, 4) if n else 0.0,
            "recommended_tier": tier,
            "top_fp_reasons": [{"bucket": b, "count": c} for b, c in fp_reasons],
        }
        summary.update(cross_summary)
        summary_rows.append(summary)

    md_out.write_text("\n".join(cards_md))
    with json_out.open("w") as f:
        for row in summary_rows:
            f.write(json.dumps(row) + "\n")

    # Summary print
    sys.stderr.write(f"\n[aggregate] wrote {md_out} ({len(detector_stats)} detectors)\n")
    sys.stderr.write(f"[aggregate] wrote {json_out}\n\n")
    sys.stderr.write("Tier summary:\n")
    tiers = Counter(row["recommended_tier"] for row in summary_rows)
    for t in ("gate-tier", "observability-tier", "needs-rework", "INSUFFICIENT_DATA"):
        if tiers.get(t):
            sys.stderr.write(f"  {t:25}  {tiers[t]} detectors\n")


if __name__ == "__main__":
    main()
