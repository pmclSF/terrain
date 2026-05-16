# RFC 0000 — Terrain 0.2.0 Product Plan

- **Status:** Accepted
- **Author:** @pmclSF
- **Created:** 2026-05-10
- **Decided:** 2026-05-10
- **Discussion:** N/A (foundational RFC; project genesis pre-dates the formal RFC process)

---

## Summary

This RFC ratifies the Terrain 0.2.0 product plan documented in `docs/PRODUCT.md` as the project's canonical product reference and accepts it as the binding scope of the 0.2.0 release. The plan establishes Terrain as the reference implementation of a new product category — *unified testing for AI/ML systems* — and commits to three co-equal product goals (unified graph, real CI gate, auditable quality), 75-rule catalog (30 stable + 45 preview), three-surface model (CI/CLI/MCP-agent), LB-1 through LB-12 quality bars, and a phased roadmap with integration philosophy.

## Motivation

This is the first RFC the project files. The RFC process documented in `docs/CONTRIBUTING.md` requires significant changes to go through this directory. The product plan itself is the most significant change in the project's history; filing it as RFC 0000 establishes:

1. **Process self-consistency.** The project commits to RFCs for significant changes; the plan is significant; therefore it has an RFC.
2. **Public record.** Future contributors who want to know "why does Terrain do X?" can read RFC 0000 and see the answer is "by design, ratified on 2026-05-10."
3. **Reference shape.** Subsequent RFCs use this RFC as the shape they're amending.

## Detailed design

The detailed design is `docs/PRODUCT.md` in its entirety. This RFC does not duplicate it; it ratifies it by reference.

Key commitments accepted by this RFC:

- **Three co-equal product goals** (PRODUCT.md §3): unified graph + real CI gate + auditable quality
- **Rule catalog:** 75 rules across 10 categories; 30 stable (default-on), 45 preview (opt-in). Stable count is a ceiling; actual stable count at release is whatever clears LB-5 (PRODUCT.md §9)
- **Three-surface model** (PRODUCT.md §7): CI surface (templates only, no LLM ever); CLI surface (`terrain test` / `terrain explain` / `terrain describe` / `terrain accept-snapshot` / `terrain init`); agent surface (MCP server pinned to spec 2025-11-25)
- **LB quality bars** (PRODUCT.md §12): LB-1 through LB-12, including LB-2a/b/c triage, LB-5 Wilson 95% lower-bound FP rate, LB-6 recall on seeded-failure corpus, LB-9 per-phase runtime, LB-11 bidirectional cause attribution, LB-12 senior-decision-maker comprehension
- **Validation harness** (PRODUCT.md §13): hand-labeled corpus of ≥100 PRs per dogfood repo; paid external triage panel; public readiness cards per release
- **Dogfood-repo set** (PRODUCT.md §14): five repos covering FE+BE RAG, Go monolith, AI-only, polyglot monorepo, classical ML pipeline
- **Project operations** (PRODUCT.md §18): Apache 2.0 license; semver with pre-1.0 deprecation cycles; zero telemetry by default; RFC governance; documented issue-triage SLOs
- **Dependency-order spine** (PRODUCT.md §16): Tier 0 (foundation) → Tier 1 (edges/adapters) → Tier 2 (rules) → Tier 3 (surfaces) → Tier 4 (validation)

## Drawbacks

- **Scope is genuinely multi-engineer-year.** The plan is honest about this; the dependency-order spine acknowledges it. Adopters and contributors should walk in with realistic expectations.
- **Triage panel funding model is recurring operational cost.** ~25 paid contractors per release cycle. Maintainer-funded for 0.2.0; longer-term funding model TBD per `docs/CONTRIBUTING.md`.
- **0.2.0 is a clean slate vs. pre-0.2.0.** Adopters using earlier Terrain treat 0.2.0 as a fresh install; no migration path is provided. The project judged this acceptable because adopter base is small (per §19 #7).
- **Several rules in stable depend on infrastructure that's net-new for 0.2.0.** The dependency-order spine sequences this; risk is dependency-chain breakage causing late-release scope cuts. Mitigated by per-rule readiness cards being independent and by the explicit "30 stable is a ceiling, not a floor" framing.

## Alternatives considered

- **Smaller 0.2.0 scope** — ship the unified-graph product without classical-ML rules; defer to 0.3.0. Rejected because the plan explicitly positions Terrain as AI *and* ML (per `docs/PRODUCT.md` §5 vocabulary section); shipping LLM-only would mis-frame the category claim.
- **Defer auditable quality (Goal 3) to 0.3.0** — ship two-goal product (unified graph + CI gate) first. Rejected because public readiness cards and open calibration corpus are the trust signals that make adopter evaluation possible; deferring them ships a product senior decision-makers can't evaluate.
- **Marketplace listings (Claude Skill, GitHub Actions Marketplace) at 0.2.0** — accelerate distribution. Rejected because the MCP server should harden through one release cycle of adopter use before locking vendor-marketplace wrappers per §16 non-goals.
- **Do nothing** — keep Terrain as the pre-0.2.0 single-domain test-quality tool it currently is. Rejected because the unified AI/ML testing category does not exist as a product today and Terrain has the architectural foundation to define it.

## Unresolved questions

All operational opens are tracked in `docs/PRODUCT.md` §19. Three remain at RFC acceptance time:

1. **License audit per dogfood-repo fork candidate** — operational; depends on shortlisting specific OSS candidates
2. **Triage panel recruitment pilot** — operational outreach
3. **Adopter base size measurement** — analytics from Homebrew / npm / GitHub stats

These do not block RFC acceptance; they block specific Tier 4 work items.

Four prior opens resolved via web research and committed in `bffd818`:
- MCP spec version 2025-11-25 confirmed latest stable
- Gauntlet reframed (Apache-2.0; tightly coupled to MosaicML stack)
- Provider matrix updated (Anthropic native preferred; Ollama tool-calling first-class; vLLM strict caveat)
- gitleaks v8 embeddability confirmed via `/detect` package

## Migration / rollout plan

This RFC ships with Terrain 0.2.0. The plan itself (`docs/PRODUCT.md`) is committed as `d27ef16`. Adopters consume the product plan via the README's "What is Terrain" pointer to `docs/OVERVIEW.md`.

There is no pre-0.2.0 migration path; 0.2.0 is the first stable release per `docs/PRODUCT.md` LB-8.

## Validation

This RFC's "did the change work?" question is whether `docs/PRODUCT.md` clears three adversarial review passes plus four codebase audits plus a verifier-confirmed redesign. All five rounds completed; 43 confirmed findings closed. Validation log:

- Round 1: initial doc adversarial review (60 findings across 6 tiers, all addressed)
- Round 2: post-audit redesign incorporating cross-stack, ML-lifecycle, test-detection, infrastructure, and existing-code findings
- Round 3: v3 technical-excellence review by four-reviewer panel + verifier sub-agent (46 candidate findings; 43 confirmed; 0 refuted; 3 gap-classified)
- Subsequent operational research closed 4 of 7 §19 open questions

LB-12 panel validation of `docs/OVERVIEW.md` is a release-tag gate, not an RFC-acceptance gate.

## Decision

**Accepted on 2026-05-10.** The plan is the binding scope of Terrain 0.2.0.

Subsequent RFCs may amend specific sections of `docs/PRODUCT.md` per the one-cycle deprecation contract; the plan itself stays a living document referenced by RFC 0000. When `docs/PRODUCT.md` is materially edited, the change is documented either via a new amendment RFC or via `CHANGELOG.md` if the change is non-significant.
