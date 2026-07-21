# Contributing to Terrain — governance and RFC process

> *This document covers the contribution process for proposing significant changes to Terrain (new rules, breaking changes, new public-API surfaces, new integrations). For day-to-day contribution mechanics (build, test, dev loop), see `/CONTRIBUTING.md` at the repo root.*

## Governance

### Maintainer

- **Project owner:** pmclSF (expand as additional maintainers join)
- **Maintainer responsibilities:** RFC review and merge; release tagging; final decisions on contested changes; communication on the project's behalf in public channels.

### RFC process

Significant changes go through GitHub RFCs in the `rfcs/` directory at the repo root before merge.

**"Significant" includes:**
- New stable rules (a rule proposed for default-on at any release)
- Breaking changes to any public API (rule IDs, JSON output schema, `terrain.yaml` schema, CLI flags, artifact format)
- New public artifacts (e.g., a new readiness-card field, a new published benchmark)
- New integrations beyond bug fixes to existing ones
- New required dependencies
- Changes to quality bars
- Changes to how rule maturity is evaluated

**"Trivial" — does *not* require an RFC:**
- Typo fixes; doc updates that don't change committed semantics
- Bug fixes to existing rules (detection refinements that improve precision without changing what the rule targets)
- Performance improvements that don't change behavior
- New preview rules where the rule's scope is incremental within an existing category

When in doubt, file an RFC. Cheap to write, cheap to decline.

### RFC structure

RFC files live at `rfcs/NNNN-short-name.md` where `NNNN` is the next available number. Use the template at `rfcs/_template.md`:

```markdown
# RFC NNNN — <Short Name>

- **Status:** Draft | Accepted | Declined | Superseded
- **Author:** @username
- **Created:** YYYY-MM-DD
- **Discussion:** <GitHub issue or PR link>

## Summary

One paragraph. What does this RFC propose?

## Motivation

Why is this change worth making? What problem does it solve?

## Detailed design

Concrete proposal. If it's a new rule, include the rule template (per `docs/rules/_template.md`). If it's a breaking change, include the deprecation cycle plan.

## Drawbacks

What does this cost? Why would someone object?

## Alternatives

What other approaches were considered?

## Unresolved questions

What's still open at draft time?
```

### Decision process

- **Draft:** author files the RFC, opens a discussion issue or PR, requests review
- **Discussion:** open for at least 7 days for community comment
- **Decision:** maintainer accepts or declines
- **Declined RFCs are kept** in the `rfcs/` directory with `Status: Declined` and rationale documented. The project has a public record of "we considered this and declined" — which serves future contributors who want to know "has this been proposed?"

Disagreements are resolved by the maintainer. Pre-1.0, the maintainer's decision is final; post-1.0 governance may evolve (steering committee, etc.) but that change itself goes through an RFC.

## Rule lifecycle

### Proposing a new rule

1. File a GitHub issue describing the rule's intent and target failure mode
2. Optionally discuss informally before authoring an RFC
3. Author the RFC including:
   - Rule template fill (per `docs/rules/_template.md`)
   - Detection mechanism
   - Default severity
   - Why it deserves to be a Terrain rule (vs. handled by an existing tool)
   - Worked example
4. Maintainer review; community comment
5. Decision

### Default tier for new rules

New rules ship as **preview** by default. Preview status is not a quality slight — preview rules ship with full detection implementations and short-form documentation. The distinction is whether the rule has been exercised enough to ship default-on with confidence. Preview rules are scope-under-evaluation; their reports from opt-in adopters help decide whether they graduate to stable.

### Graduation to stable

A preview rule graduates to stable when:

1. The rule has been exercised enough to ship default-on with confidence, and its documented maturity reflects that.
2. The rule's doc page is filled to full stable-tier (sections 1–11, ~800–1500 words)
3. A graduation RFC is filed (lightweight)
4. Maintainer accepts the graduation

Graduations are batched per release. Some rules may never graduate (their fundamental mechanism stays imprecise or category remains experimental); the project documents this rather than holding releases.

### Deprecating a rule

Rules are deprecated when:
- Their detection mechanism has been replaced by a better rule (the new rule subsumes the old)
- Their FP rate has degraded post-graduation and tuning hasn't restored it
- The capability they detect is no longer relevant (e.g., a vendor-specific rule for a deprecated framework)

Deprecation follows the one-cycle process:
1. RFC documents the deprecation rationale and replacement (if any)
2. Rule emits a stderr warning when fired: `WARNING: rule terrain/<id> is deprecated; replaced by terrain/<new-id>. Removal in v<next-minor>.`
3. After one minor release, the rule is removed; the rule's doc page redirects to its replacement

## Issue triage and response

Targets:

| Issue type | Acknowledgment target | Fix target |
|---|---|---|
| Security vulnerability | 24 hours | 7 days (critical) / 30 days (high) |
| Bug in stable rule (false positive at scale; false negative) | 48 hours | best-effort, prioritized by impact |
| Bug in preview rule | 1 week | best-effort, may be addressed via rule revision |
| Feature request | 1 week | discussed via RFC process |
| Documentation issue | 48 hours | best-effort within minor release cycle |

**These are targets, not guarantees.** They scale with maintainer capacity. Pre-1.0, the project commits to publishing actual response-time data per release in `CHANGELOG.md`. Adopters can hold the project accountable to these targets via public artifacts.

## Code-level contribution mechanics

For everything else (development environment, building, running tests, linting, PR conventions), see the root `/CONTRIBUTING.md`.

## What contributions are most needed

- **Bug reports with reproducers.** A reproducer is worth a thousand prose descriptions of "X is broken."
- **Real-world failure-mode documentation.** If you hit a Terrain rule with a counterexample (a false positive or a missed detection), file an issue with the input — these directly help improve the rule.
- **Adopter feedback on preview rules.** Opt into a preview rule, run it on real code, report what you see. This is the primary signal that helps a rule graduate.
- **Eval-framework adapter contributions.** If you use an eval framework not yet supported and want Terrain to consume its output, an RFC-then-adapter contribution is welcome.

## What contributions are unlikely to be accepted

- **Vendor-specific rules for proprietary tools.** Unless the vendor commits to an open spec / artifact format, Terrain doesn't ship per-vendor rules.
- **Rules that duplicate existing OSS scanning tools.** Terrain integrates with gitleaks, Presidio, etc.; it doesn't reimplement them. New `security/*` rules should add value beyond what the underlying scanner provides.
- **Marketing / positioning changes** that aren't tied to a substantive product change. `docs/OVERVIEW.md` and `docs/LIMITATIONS.md` evolve carefully; large rewrites should be RFCs.
- **Hosted-service or paid-tier proposals.** Terrain is OSS that runs in adopter infrastructure; this is a categorical commitment.

---

*If you're unsure whether your contribution warrants an RFC, file an issue first and ask. The maintainer would rather discuss before code is written than decline a complete PR.*
