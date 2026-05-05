#!/usr/bin/env python3
"""Apply the parity-cell lifts the 0.2.0 merge wave delivered.

This is a one-shot updater. Each entry in LIFTS names an (area, axis)
cell, the new score, and the evidence (which merged PR delivered the
lift). Running this script rewrites docs/release/parity/scores.yaml
in place.

The approach is line-based string replacement on the specific line
matching each cell's `<axis>: { score: <N>, ...` shape; we do NOT
parse YAML to keep diffs minimal and reviewable.
"""
import re
from pathlib import Path

LIFTS = [
    # core_analyze — Track 10.1 (uitokens), 9.1 (capability metadata),
    # 9.4 (budget), 9.3 (missing-input diags), 5.3 (ctx audit)
    ("core_analyze", "V1", 3, "Track 10.1 — internal/uitokens/ design tokens shipped (#136). Renderers haven't migrated yet (Track 10.2 is 0.2.x); foundation exists."),
    ("core_analyze", "V3", 3, "Track 10.6 — internal/reporting/empty_states.go shipped (#149) with 7 designed empty-state kinds + voice/tone tests. Not yet wired into every callsite; foundation exists."),
    ("core_analyze", "E3", 4, "Track 9.1 capability metadata + Track 9.3 missing-input diags (#155) surface per-detector requirements; Track 9.4 detector budgets (#154) emit budget markers when exceeded; Track 5.6 per-component timing in --verbose."),
    ("core_analyze", "E7", 4, "Track 5.3 ctx audit on AI detectors (#146) — DetectContext respects ctx in inner loops with deliberately-slow-detector test. Track 9.4 budget enforcement (#154) provides safety net for unaware detectors."),
    ("core_analyze", "P3", 4, "Track 9.12 schema field tier doc (#151), Track 7.3 trust-boundary doc (#144), Track 6.5 alignment-first migration framing (#148), Track 9.7 truth-verify gate (#156)."),
    ("core_analyze", "P6", 4, "Track 1.6 docs/examples/{understand,align,gate}/ shipped via #137 + Track 7.4 Promptfoo+Terrain CI walkthrough (#147) + Track 6.4 multi-repo example (#152)."),
    ("core_analyze", "E1", 5, "Track 9.9 adversarial filesystem suite (#150) with 8 hostile inputs (binary files, BOM, NUL bytes, nested .git, deep nesting). Track 9.11 schema migration fixture (#150). Existing unit + integration + golden coverage retained."),

    # insights_impact_explain
    ("insights_impact_explain", "V1", 3, "Track 10.1 uitokens shipped (#136). Inherits foundation."),
    ("insights_impact_explain", "V3", 3, "Track 10.6 empty-state helpers (#149) including EmptyNoImpact / EmptyNoTestSelection / EmptyZeroFindings."),
    ("insights_impact_explain", "E3", 3, "Track 3.2 --explain-selection per-test reason chains (#157) gives the 'why these tests, why not others' view. Track 9.4 detector budgets surface timing breakdowns."),
    ("insights_impact_explain", "P1", 4, "Track 4.6 terrain explain finding <id> (#140). Track 3.2 --explain-selection (#157)."),

    # summary_posture_metrics_focus
    ("summary_posture_metrics_focus", "V1", 3, "Inherits Track 10.1 uitokens (#136)."),
    ("summary_posture_metrics_focus", "V3", 3, "Track 10.6 empty-state helpers (#149)."),

    # pr_change_scoped (Gate pillar — needs ≥4)
    ("pr_change_scoped", "P1", 4, "Track 3.1 report pr --fail-on (#157). Track 4.8 --new-findings-only --baseline (#140). Track 3.2 --explain-selection (#157). Track 4.5 suppressions (#158) honored across pr."),
    ("pr_change_scoped", "P3", 4, "Track 4.5 suppressions docs (#158). Track 3.5 unified-pr-comment.md visual contract (#145). Track 6.6 alignment-first migration framing (#148)."),
    ("pr_change_scoped", "P4", 4, "Track 8.5 single recommended GitHub Action template w/ safe defaults (#142). Track 8.6 trust ladder (#142). Track 7.4 end-to-end Promptfoo+Terrain CI walkthrough (#147)."),
    ("pr_change_scoped", "P6", 4, "Track 1.6 examples/, Track 7.4 ai-eval-ci/ walkthrough (#147), Track 6.4 multi-repo example (#152). Visual regression goldens (#149)."),
    ("pr_change_scoped", "E3", 3, "Track 3.2 explain-selection reason chains (#157). Track 9.4 per-detector budget timing visibility (#154). Confidence histogram still missing — would lift to 4."),
    ("pr_change_scoped", "V1", 3, "Track 10.1 uitokens shipped (#136). PR-comment migration to tokens is Track 10.3 (0.2.x)."),
    ("pr_change_scoped", "V3", 3, "Track 10.6 empty states designed (#149). PR-empty-no-findings template documented in Track 3.5 unified-pr-comment.md (#145)."),

    # ai_risk_inventory
    ("ai_risk_inventory", "P3", 4, "Track 5.1 ai-risk-tiers.md (#146) documents the inventory/hygiene/regression subdivision. Track 7.3 trust-boundary (#144). Per-detector docs/rules/ai/ pages."),
    ("ai_risk_inventory", "V1", 3, "Track 5.1 AISubdomainOf / AISubdomainTrustBadge helpers (#146). Renderers consume from one source. uitokens migration is 0.2.x."),
    ("ai_risk_inventory", "V2", 3, "Track 5.1 inventory/hygiene/regression sub-stanzas in PR comment (#146 + #145 unified shape)."),
    ("ai_risk_inventory", "E7", 4, "Track 5.3 ctx audit on AI detectors (#146): DetectContext respects ctx with deliberately-slow-detector test. Track 9.4 budget enforcement (#154)."),
    ("ai_risk_inventory", "P6", 3, "Track 1.6 examples include AI surfaces. Track 7.4 ai-eval-ci/ walkthrough (#147)."),

    # ai_eval_ingestion (Gate — needs 4)
    ("ai_eval_ingestion", "P3", 4, "Track 7.3 trust-boundary (#144). Track 7.4 ai-eval-ci/ walkthrough (#147). Track 5.1 ai-risk-tiers (#146)."),
    ("ai_eval_ingestion", "P6", 4, "Track 7.1 conformance fixtures per (framework × version) (#147) — 8 fixtures across Promptfoo v3/v4, DeepEval 1.x, Ragas modern/legacy."),
    ("ai_eval_ingestion", "E1", 4, "Track 7.1 conformance test suite (#147) covering canonical + drift shapes per framework."),
    ("ai_eval_ingestion", "E2", 3, "Track 7.2 ShapeInfo + warn-on-drift (#147) surfaces unfamiliar shapes. Real-PR precision corpus (Track 7.5) deferred — labeled data needed."),
    ("ai_eval_ingestion", "V1", 3, "Inherits Track 10.1 uitokens (#136). Adapter outputs flow through unified PR comment (#145)."),
    ("ai_eval_ingestion", "V3", 3, "Track 10.6 empty states (#149) — no-eval-results path documented."),

    # ai_execution_gating (Gate — needs 4)
    ("ai_execution_gating", "P3", 4, "Track 7.3 trust-boundary doc (#144) — what executes vs parses, per-command surface, sandboxing roadmap."),
    ("ai_execution_gating", "P6", 4, "Track 7.4 end-to-end Promptfoo+Terrain CI walkthrough (#147)."),
    ("ai_execution_gating", "V1", 3, "Inherits uitokens (#136)."),

    # migration_conversion (Align — soft, ≥3)
    ("migration_conversion", "P3", 4, "Track 6.5 alignment-first migration framing doc (#148). Track 6.6 tier badges in migrate list (#148)."),
    ("migration_conversion", "P4", 4, "Track 6.6 Stable / Experimental / Preview tier vocabulary surfaced in `migrate list` output (#148)."),
    ("migration_conversion", "V1", 3, "Inherits uitokens (#136)."),

    # portfolio (Align — soft, ≥3)
    ("portfolio", "P1", 3, "Track 6.1 multi-repo manifest format ships (#148): RepoManifest schema v1, loader, validator, path resolution. Aggregator (6.2/6.3) is 0.2.x."),
    ("portfolio", "P3", 3, "Track 6.4 docs/examples/align/multirepo/ with full convergence story (#152). Manifest schema doc."),
    ("portfolio", "P4", 3, "Manifest format adopters can write today; 0.2.x aggregator consumes unchanged."),
    ("portfolio", "P6", 3, "Track 6.4 multi-repo example fixture (#152) shows expected output shape."),
    ("portfolio", "E1", 3, "12 manifest validation tests (#148) cover canonical + every error path + path resolution variants."),
    ("portfolio", "V1", 3, "Inherits uitokens (#136)."),

    # policy_governance (Gate — needs 4)
    ("policy_governance", "P4", 4, "Track 8.4 init policy template (#142). Track 7.6/7.7 example policies (#141)."),
    ("policy_governance", "P6", 4, "Three example policies ship (#141): minimal / balanced / strict."),
    ("policy_governance", "V1", 3, "Inherits uitokens (#136)."),

    # server (Understand — needs 3)
    ("server", "V1", 3, "Inherits uitokens (#136)."),
    ("server", "V2", 3, "Inherits PR #130 visual polish."),
    ("server", "V3", 3, "Track 10.6 empty states (#149) include 'no AI surfaces' and 'no policy file'."),
    ("server", "E3", 3, "Track 9.1 capability metadata observability (#155). Per-component timing extends to server endpoints."),
    ("server", "E6", 3, "Inherits snapshot determinism."),
    ("server", "P5", 3, "Server-context cancellation + dedup (#132)."),
    ("server", "E5", 3, "Server uses singleflight per #132 to avoid duplicate concurrent analyses."),

    # distribution_install (cross-cutting — soft)
    ("distribution_install", "P3", 4, "Track 8.1 Node 22 prominence in README + getting-started (#144). Track 9.12 schema field tiers (#151)."),
    ("distribution_install", "P4", 3, "Track 8.5 GitHub Action template + safe-default mode (#142). Track 8.1 brew/go install fallback documented for Node-20 CI (#144)."),
    ("distribution_install", "E3", 3, "Track 8.2 release-smoke matrix expanded to darwin/arm64 + windows/amd64 (#144)."),
    ("distribution_install", "V1", 3, "Inherits uitokens (#136)."),
    ("distribution_install", "V2", 3, "Trust ladder doc (#142) provides scannable adoption progression."),
]


def main():
    path = Path("docs/release/parity/scores.yaml")
    text = path.read_text()
    applied = 0

    for area, axis, score, evidence in LIFTS:
        # Match within an area block: the area header is `  <area>:` and
        # the cell line is `    <axis>: { score: ..., ... }`. Simpler to
        # do a per-line walk than a full parse.
        lines = text.split("\n")
        in_area = False
        for i, line in enumerate(lines):
            stripped = line.strip()
            if not in_area:
                if stripped == f"{area}:":
                    in_area = True
                continue
            # We're inside the target area. Stop when we hit another
            # area header (2-space-indented and ends in `:` with no
            # value).
            if line.startswith("  ") and not line.startswith("    ") and line.rstrip().endswith(":"):
                break
            m = re.match(rf"^(\s+){axis}:\s*\{{\s*score:\s*\d+", line)
            if m:
                indent = m.group(1)
                # Escape evidence for YAML inline-string safety: use
                # double quotes; escape any double-quote in the body.
                ev_esc = evidence.replace('"', '\\"')
                lines[i] = f'{indent}{axis}: {{ score: {score}, evidence: "{ev_esc}" }}'
                applied += 1
                break
        else:
            # Loop completed without break — area not found at all.
            print(f"WARN: area {area!r} not found in scores.yaml")
            continue

        text = "\n".join(lines)

    path.write_text(text)
    print(f"Applied {applied}/{len(LIFTS)} lifts.")


if __name__ == "__main__":
    main()
