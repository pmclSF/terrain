# Triage panel — volunteer recruitment, protocol, cadence

The triage panel measures LB-2a (triage decision ≤60s P75), LB-2b (fix-direction P75 per-category), LB-2c (agent-surface usability), and LB-12 (senior-decision-maker comprehension). Without a real panel running, those LBs can't be measured, and the gate can't ship with honest readiness cards.

Per `docs/PRODUCT.md` §6 *solo execution* and §13, the 0.2.0 panel is a **volunteer model with small honoraria**, not full paid contractors. This document is the operational playbook.

## Panel composition

- **Target size:** ~10 engineers (down from earlier draft's 25; sized for solo-execution funding constraint)
- **Per-rule sample:** N=3 (down from N=5; readiness cards report N alongside the measured value so adopters know the statistical confidence)
- **Eligibility:** familiar with at least one of (ESLint, Clippy, mypy, Cargo, similar lint/test-quality tools); *not* familiar with Terrain itself
- **Sourcing:** OSS developer-tools community — Hacker News, Lobsters, dev-Twitter, relevant subreddits, tool-specific user groups. Volunteer-first; honoraria are recognition, not compensation
- **Rotation:** each volunteer participates in 1-2 sessions per release cycle, ~30-60 min per session

## Honoraria

- **Not paid-contractor rates.** Recognition-level: $25-50 gift cards per session, or equivalent (e.g., a year of a relevant SaaS dev tool, sponsorship of an OSS project of the volunteer's choice)
- **Maintainer's personal budget** at 0.2.0; total ceiling per release cycle: ~$500-1000 estimated for the first run
- **Optional for volunteers** — many community members will participate without compensation; honoraria are offered, not required

## Recruitment process

1. **Outreach copy** (TODO before first run): draft a 2-paragraph description of what the panel is, time commitment, what's measured, the honoraria offer. Post once on Hacker News (Show HN), Lobsters, dev-Twitter, relevant subreddits
2. **Application:** GitHub-issue-based "Triage panel: round 1" tracker; volunteers comment with relevant experience (lint/test-quality tool background) and availability
3. **Screening:** maintainer reviews comments, selects ~10 volunteers across rule-categories of expertise
4. **Onboarding:** 5-min Terrain orientation (Loom or doc); confirms understanding before timing begins
5. **No NDA required.** Sessions cover OSS dogfood-repo findings, not adopter code

## Per-session protocol (LB-2a/b)

Each volunteer sees a session bundle:

- Short Terrain orientation (5 min): "Terrain is a CI gate; these are findings; here's the diagnostic format"
- N findings drawn from a stratified sample (across rule categories and dogfood repos). One panelist sees ~5-8 findings per session
- For each finding, the panelist answers:
  - **"Is this a true positive or false positive?"** (LB-2a — decision in ≤60s P75)
  - If TP: **"What's a candidate fix direction?"** (LB-2b — articulation in ≤Ns per category target)

Sessions are self-timed by the panelist; the panelist records their own start/stop timestamps. Maintainer grades panelist answers against the rule's ground-truth label (from `harness/corpora/<repo>/labels.yaml`) for correctness, and reports median + P75 + P90 from the timestamps for each rule.

## Per-session protocol (LB-2c — agent surface)

Separately, a smaller volunteer panel (N=3 per release):

- 5-min Terrain orientation + 5-min Claude Code (or Cursor) orientation with the MCP server installed
- One real failing Terrain finding from the dogfood repos
- Asked to "resolve it" via the agent's conversation

Success: the panelist proposes a candidate fix within a 10-min session, graded correct against the rule's ground-truth label.

## Per-session protocol (LB-12 — senior decision-maker)

Once per release, a senior-tier volunteer panel (N=3 minimum: senior engineers / engineering managers / PMs unfamiliar with Terrain):

- Reads `docs/OVERVIEW.md` + 3 sample readiness cards + `docs/LIMITATIONS.md` (~10-15 min reading)
- Asked 4 questions:
  1. What category of tool is this?
  2. What trust profile does it commit to?
  3. What would adopting it require of my team?
  4. What could break under the stability contract?

Bar: ≥2 of 3 panelists answer all four correctly (adjusted from earlier ≥4 of 5 for the smaller panel size).

## Cadence

- Once per release tag at minimum (0.2.0, 0.3.0, etc.)
- Spot checks per patch release only when a stable rule's detection mechanism changes
- Per `docs/PRODUCT.md` §13: pre-1.0, the project publishes actual panel-run dates and outcomes in each release's readiness cards. Adopters can verify the cadence is being followed.
- **If volunteer recruitment fails to deliver minimum N** (≥3 per rule for LB-2, ≥3 senior for LB-12): cadence drops with explicit acknowledgment in release notes; the affected LBs are marked "panel-pending" in readiness cards rather than measured to inflated confidence

## Privacy

- Volunteer identities are not disclosed in readiness cards (aggregate measurements only)
- Session content is from dogfood repos (project-owned bespoke or permissive-licensed forks), not adopter code
- Session recordings (if any) are kept internal to maintainer for grading purposes; not published

## TODO before first panel run

- [ ] Draft outreach copy (HN post, social-media variants)
- [ ] Pick honorarium type (gift card vendor, SaaS subscription, OSS-sponsorship route) and budget cap
- [ ] Set up GitHub-issue template for the "Triage panel: round 1" recruitment thread
- [ ] Finalize session bundle format (a static HTML page with findings + ground-truth labels, or a GitHub Gist per session)
- [ ] Draft grader rubric for ambiguous TP/FP determinations (binary correctness, but some findings have nuance)
- [ ] Decide first-run sampling protocol (stratified by rule category and dogfood repo)

## Post-1.0 expansion

When the project has a stable adopter base and (potentially) sponsorship / foundation funding, the panel can scale:

- Larger N per rule (back to N=5+)
- Full paid-contractor rates
- More frequent cadence
- More diverse panelist pool

The 0.2.0 model is appropriate for the project's current size; expanding the panel as adoption grows is a normal sign of the project succeeding.
