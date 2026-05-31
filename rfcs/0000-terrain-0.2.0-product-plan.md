# RFC 0000 — Terrain 0.2.0 Product Plan

- **Status:** Accepted
- **Author:** @pmclSF
- **Created:** see git log
- **Decided:** see git log
- **Discussion:** N/A (foundational RFC; project genesis pre-dates the formal RFC process)

---

## Summary

This RFC ratifies the Terrain 0.2.0 product plan documented in `docs/PRODUCT.md` as the project's canonical product reference and accepts it as the binding scope of the 0.2.0 release. The plan establishes Terrain as the reference implementation of a new product category — *unified testing for AI/ML systems* — and commits to three co-equal product goals (unified graph, real CI gate, auditable quality), a rule catalog spanning ten categories with stable / preview tiers, a three-surface model (CI/CLI/MCP-agent), measured quality bars, and an integration philosophy.

## Motivation

This is the first RFC the project files. The RFC process documented in `docs/CONTRIBUTING.md` requires significant changes to go through this directory. The product plan itself is the most significant change in the project's history; filing it as RFC 0000 establishes:

1. **Process self-consistency.** The project commits to RFCs for significant changes; the plan is significant; therefore it has an RFC.
2. **Public record.** Future contributors who want to know "why does Terrain do X?" can read RFC 0000 and see the answer is "by design, ratified in this RFC."
3. **Reference shape.** Subsequent RFCs use this RFC as the shape they're amending.

## Detailed design

The detailed design is `docs/PRODUCT.md` in its entirety. This RFC does not duplicate it; it ratifies it by reference.

Key commitments accepted by this RFC:

- **Three co-equal product goals**: unified graph + real CI gate + auditable quality
- **Rule catalog** spanning ten categories with stable (default-on) and preview (opt-in) tiers. Stable count is a ceiling; actual stable count at release is whatever clears the quality bars
- **Three-surface model**: CI surface (templates only, no LLM ever); CLI surface (`terrain test` / `terrain explain` / `terrain describe` / `terrain accept-snapshot` / `terrain init`); agent surface (MCP server pinned to spec 2025-11-25)
- **Quality bars**: measured false-positive rate, recall on seeded-failure fixtures, per-phase runtime, bidirectional cause attribution, senior-decision-maker comprehension
- **Validation harness**: representative-repository coverage with published readiness cards per release
- **Project operations**: Apache 2.0 license; semver with pre-1.0 deprecation cycles; zero telemetry by default; RFC governance; documented issue-triage SLOs
- **Dependency-order spine**: foundation → edges/adapters → rules → surfaces → validation

## Drawbacks

- **Scope is genuinely multi-engineer-year.** The plan is honest about this; the dependency-order spine acknowledges it. Adopters and contributors should walk in with realistic expectations.
- **Validation operations carry recurring cost.** Funding model is documented in `docs/CONTRIBUTING.md`.
- **0.2.0 is a clean slate vs. pre-0.2.0.** Adopters using earlier Terrain treat 0.2.0 as a fresh install; no migration path is provided. The project judged this acceptable because the adopter base was small at decision time.
- **Several rules in stable depend on infrastructure that's net-new for 0.2.0.** The dependency-order spine sequences this; risk is dependency-chain breakage causing late-release scope cuts. Mitigated by per-rule readiness cards being independent and by the explicit "30 stable is a ceiling, not a floor" framing.

## Alternatives considered

- **Smaller 0.2.0 scope** — ship the unified-graph product without classical-ML rules; defer to 0.3.0. Rejected because the plan explicitly positions Terrain as AI *and* ML (see the vocabulary section of `docs/PRODUCT.md`); shipping LLM-only would mis-frame the category claim.
- **Defer auditable quality (Goal 3)** — ship two-goal product (unified graph + CI gate) first. Rejected because public readiness cards are the trust signals that make adopter evaluation possible; deferring them ships a product senior decision-makers can't evaluate.
- **Marketplace listings (Claude Skill, GitHub Actions Marketplace) at 0.2.0** — accelerate distribution. Rejected because the MCP server should harden through one release cycle of adopter use before locking vendor-marketplace wrappers; see the non-goals section in `docs/PRODUCT.md`.
- **Do nothing** — keep Terrain as the pre-0.2.0 single-domain test-quality tool it currently is. Rejected because the unified AI/ML testing category does not exist as a product today and Terrain has the architectural foundation to define it.

## Unresolved questions

All operational opens are tracked in `docs/PRODUCT.md` (Open questions). Three remain at RFC acceptance time:

1. **License audit per representative-repository candidate** — operational; depends on shortlisting specific OSS candidates
2. **Validation operations onboarding** — operational outreach
3. **Adopter base size measurement** — analytics from Homebrew / npm / GitHub stats

These do not block RFC acceptance; they block specific Tier 4 work items.

Four prior opens resolved via web research:
- MCP spec version 2025-11-25 confirmed latest stable
- Gauntlet reframed (Apache-2.0; tightly coupled to MosaicML stack)
- Provider matrix updated (Anthropic native preferred; Ollama tool-calling first-class; vLLM strict caveat)
- gitleaks v8 embeddability confirmed via `/detect` package

## Migration / rollout plan

This RFC ships with Terrain 0.2.0. Adopters consume the product plan via the README's "What is Terrain" pointer to `docs/OVERVIEW.md`.

There is no pre-0.2.0 migration path; 0.2.0 is the first stable release per `docs/PRODUCT.md`.

## Validation

This RFC's "did the change work?" question is whether `docs/PRODUCT.md` clears its review process. All review iterations completed and confirmed findings are closed.

Senior-decision-maker validation of `docs/OVERVIEW.md` is a release-tag gate, not an RFC-acceptance gate.

## Decision

**Accepted.** The plan is the binding scope of Terrain 0.2.0.

Subsequent RFCs may amend specific sections of `docs/PRODUCT.md` per the one-cycle deprecation contract; the plan itself stays a living document referenced by RFC 0000. When `docs/PRODUCT.md` is materially edited, the change is documented either via a new amendment RFC or via `CHANGELOG.md` if the change is non-significant.
