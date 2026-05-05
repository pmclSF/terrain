# AI risk: three trust tiers

How Terrain classifies AI-domain signals into Inventory, Hygiene,
and Regression — and why adopters should see the three tiers as
visually distinct sections rather than one undifferentiated list.

## Why three tiers, not one

The launch-readiness review for 0.2 flagged an honest framing
problem: presenting AI inventory data (which models exist, which
prompts are declared) alongside heuristic AI hygiene findings
(prompt-injection structural patterns) and eval-framework-driven
regression detection (cost trends, hallucination-rate changes) as
one undifferentiated list overstates the trust we can claim.

Inventory is ground truth — Terrain reads what's declared. Hygiene
is a heuristic with documented false-positive patterns. Regression
is downstream of the eval framework's metadata, which Terrain
reads but doesn't produce.

Bundling all three under "AI Risk Review" with one severity
hierarchy made the heuristic side look as authoritative as the
inventory side. Track 5.1 of the 0.2 release plan addressed this
by classifying every AI signal into one of three subdomains and
surfacing the classification at render time.

## The three tiers

### Tier 1 — Inventory

**Trust posture:** high. Source data is ground truth (declared
surfaces, code structure, framework configs).

**Public claim:** Terrain claims Inventory data publicly in 0.2.0.
The recommended `--fail-on critical` CI gate fires on missing-eval
findings in this tier.

**Signals:**

- `aiPolicyViolation` — declared policy, declared violation
- `aiPromptVersioning` — declared prompt without versioning metadata
- `aiSafetyEvalMissing` — declared prompt surface with no safety eval scenario
- `uncoveredAISurface` — declared AI surface (model/prompt) with zero test coverage
- `untestedPromptFlow` — declared prompt flow with no scenario covering it
- `capabilityValidationGap` — declared capability with no eval scenario
- `phantomEvalScenario` — eval scenario references a surface that doesn't exist

### Tier 2 — Hygiene

**Trust posture:** medium. Detector reads source code and flags
structural shapes; precision varies by codebase.

**Public claim:** Visible in `analyze` and `report pr` output, but
**excluded from the recommended `--fail-on critical` path** in
0.2.0. Adopters can opt-in once they've measured precision in their
own repo.

**Signals:**

- `aiPromptInjectionRisk` — user input concatenated into prompt without visible sanitization
- `aiHardcodedAPIKey` — API-key-shaped literal in source
- `aiToolWithoutSandbox` — destructive-verb tool name without sandbox / approval marker
- `aiModelDeprecationRisk` — deprecated model string (text-davinci-*, etc.)
- `aiFewShotContamination` — eval test data leaks into few-shot examples
- `contextOverflowRisk` — prompt assembly likely exceeds token budget

False-positive guidance per detector lives in
[`docs/rules/ai/accuracy-regression.md`](../rules/ai/accuracy-regression.md)
and the sibling pages. Read the relevant one before opting any
hygiene signal into your blocking-gate config.

### Tier 2 — Regression

**Trust posture:** medium. Source data is the eval framework's
metadata; Terrain reads it.

**Public claim:** Same posture as hygiene — visible but not
gating-critical. Lifts to publicly claimable when paired with eval
artifacts in CI.

**Signals (eval-output-driven):**

- `aiCostRegression`, `costRegression` — token / dollar cost trends
- `aiHallucinationRate`, `hallucinationDetected` — eval-flagged factuality regressions
- `aiRetrievalRegression`, `retrievalMiss`, `topKRegression`,
  `rerankerRegression`, `chunkingRegression` — retrieval-quality
  trends
- `aiEmbeddingModelChange` — embedding model swap detected between runs
- `aiNonDeterministicEval` — eval config doesn't pin temperature / seed
- `accuracyRegression`, `latencyRegression`, `safetyFailure`,
  `evalFailure`, `evalRegression` — eval scoreboard trends
- `agentFallbackTriggered`, `toolRoutingError`,
  `toolSelectionError`, `toolGuardrailViolation`,
  `toolBudgetExceeded` — agent / tool runtime signals from eval
  metadata
- `answerGroundingFailure`, `citationMissing`,
  `citationMismatch`, `staleSourceRisk` — RAG grounding signals
- `schemaParseFailure`, `wrongSourceSelected` — pipeline metadata

These fire only when the corresponding eval artifact is present
(via `terrain ai run` or `--ingest-only`). On a repo without eval
output, Terrain silently emits zero of these — that's the contract.

## How this surfaces in output

The three tiers appear as visually distinct sub-sections in the AI
Risk Review stanza of the PR comment, each with its own trust-tier
badge:

```
### AI Risk Review

#### [Tier 1] Inventory
- **`src/agent/prompt.ts`** — declared prompt has no eval coverage
  → add an eval scenario in `evals/agent.yaml`

#### [Tier 2] Hygiene
- **`src/agent/login.ts:42`** — user input concatenated into prompt
  → wrap input through a sanitizer

#### [Tier 2] Regression
- **`evals/agent/refund.yaml`** — hallucination rate up 3.2pp vs baseline
  → review failing scenarios in the eval framework's report
```

The badges (`[Tier 1]` / `[Tier 2]`) and section labels
(`Inventory` / `Hygiene` / `Regression`) come from the public
helpers in `internal/signals/ai_subdomain.go`:

- `AISubdomainOf(signalType) → AISubdomain`
- `AISubdomainLabel(subdomain) → string`
- `AISubdomainTrustBadge(subdomain) → string`

Renderers consume these consistently; no rendering site invents
its own tier vocabulary.

## How this affects gating

The recommended CI config in
[`docs/examples/gate/github-action.yml`](../examples/gate/github-action.yml)
ships with `--fail-on critical` — and *Terrain restricts critical
severity to Tier 1 (Inventory) signals by default in 0.2.0*.
Hygiene and regression signals can ship at high severity in
output, but they don't escalate to critical (and therefore don't
block merges) unless an adopter explicitly opts them into the
critical-severity bucket via policy.

The trust posture is the contract:

> Inventory is reliable enough to publicly claim and gate on.
> Hygiene and regression are visible — actionable, surfaced in
> the comment — but not gating-critical until you've measured
> precision in your own repo or paired regression with reliable
> eval artifacts.

This is the structural alternative to the 0.1.x posture, which
gated on every AI signal at face value and consequently lost
adopter trust the first time prompt-injection over-fired on a
benign template literal.

## Adding a new AI signal

1. Add the constant to `internal/signals/signal_types.go`
2. Add the manifest entry in `internal/signals/manifest.go`
   (Domain: `models.CategoryAI`)
3. **Classify the new signal in `internal/signals/ai_subdomain.go`** —
   pick Inventory / Hygiene / Regression
4. The drift gate test (`TestAISubdomain_AllAISignalsClassified`)
   will fail CI if you skip step 3, surfacing the gap

The drift gate is the contract: every AI signal in the manifest is
classified, no exceptions, before the change can merge.

## Out of scope (0.3+)

- Configurable per-tier severity floors (today the floor is
  hardcoded: Tier 1 may be critical; Tier 2 caps at high)
- Per-detector precision corpora that lift specific Tier-2
  signals to Tier 1 with measured evidence
- Renderer-level grouping in the legacy non-markdown output
  modes (terminal-text, SARIF) — only the PR-comment markdown
  enforces the three-section grouping in 0.2.0

## Related reading

- [`docs/product/ai-trust-boundary.md`](ai-trust-boundary.md) —
  the wider question of what Terrain executes vs parses
- [`docs/product/unified-pr-comment.md`](unified-pr-comment.md) —
  how the AI Risk Review section fits the unified visual contract
- [`docs/release/feature-status.md`](../release/feature-status.md) —
  per-capability tier in the public claim matrix
- [`internal/signals/ai_subdomain.go`](../../internal/signals/ai_subdomain.go) —
  the classification map
