# RFC NNNN — <Short Name>

- **Status:** Draft | Accepted | Declined | Superseded
- **Author:** @username
- **Created:** YYYY-MM-DD
- **Decided:** YYYY-MM-DD (filled when status moves out of Draft)
- **Discussion:** <GitHub issue or PR link>
- **Supersedes:** RFC NNNN (if applicable)

---

## Summary

One paragraph. What does this RFC propose? Optimized to be read in 30 seconds — if the reader stops here, they should know enough to decide if it's worth reading the rest.

## Motivation

Why is this change worth making? What problem does it solve? Reference concrete incidents, adopter requests, or product-plan sections where the gap exists.

If this RFC fixes something in `docs/PRODUCT.md`, cite the section and quote the relevant sentence.

## Detailed design

Concrete proposal. Be specific:

- If proposing a new rule: include the full rule template fill per `docs/rules/_template.md`, including detection mechanism, default severity, worked example
- If proposing a breaking change to a public API: include the deprecation plan per the versioning section in `docs/PRODUCT.md` (alias period, stderr message format, removal version)
- If proposing a new integration: include the `docs/integrations/<tool>.md` template fill
- If proposing a new LB quality bar: include how it's measured by the validation harness

Show concrete code / config / schema where applicable. Pseudo-code is fine but be explicit about the shape of the change.

## Drawbacks

What does this cost? Why would someone reasonably object?

- Maintenance burden
- Adopter migration cost
- Conflict with existing principles or rules
- Risk of false positives or false negatives (for detection rules)
- Expansion of scope into territory the plan explicitly declined

## Alternatives

What other approaches were considered? Why is this the chosen one?

- Alternative A — why rejected
- Alternative B — why rejected
- Do-nothing — what happens if we don't do this

## Unresolved questions

What's still open at draft time? Use this section to invite discussion on specific points rather than leaving them implicit.

- Open question 1 — what answer would change the design?
- Open question 2 — same

## Migration / rollout plan

If this RFC ships in a release, what's the rollout?

- What changes in the release notes?
- What `CHANGELOG.md` entry is required?
- Is there a deprecation alias period? How long?
- How do existing adopters discover the change?

## Validation

How do we know the change worked? What harness measurements or release gates validate this?

- For rules: which quality bars apply, what's the target
- For breaking changes: how do we verify the deprecation cycle is complete
- For integrations: what end-to-end test confirms the integration works on a representative repo

## Decision (filled when status moves out of Draft)

If accepted: brief rationale for acceptance + any conditions attached.

If declined: brief rationale for declining + what would change the answer in the future.

The project keeps **all** RFCs in `rfcs/` including declined ones, with their decision rationale. This creates a public record of considered-and-declined ideas so future contributors don't re-litigate.
