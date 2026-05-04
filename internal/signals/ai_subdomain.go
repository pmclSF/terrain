package signals

import "github.com/pmclSF/terrain/internal/models"

// AISubdomain classifies an AI-domain signal into one of three trust
// tiers. The subdivision is the load-bearing change for Track 5.1 of
// the 0.2 release plan: the launch-readiness review flagged that
// presenting AI inventory data alongside heuristic AI hygiene and
// eval-data-dependent regression signals as a single undifferentiated
// list overstated the trust we can claim.
//
// Adopters reading the AI Risk Review section of a PR comment should
// see — at a glance — which signals are derived from facts (inventory),
// which are heuristic structural patterns (hygiene), and which depend
// on eval-framework metadata (regression). The three tiers correspond
// to the public-claimability bar:
//
//   - Inventory  → Tier 1: claimed publicly in 0.2.0
//   - Hygiene    → Tier 2: visible but not on the recommended
//                  --fail-on path; opt-in for adopters who've
//                  measured precision in their own repo
//   - Regression → Tier 2: same posture as hygiene, but the
//                  data-quality lever is "do you ship eval
//                  artifacts" rather than "do you accept heuristic
//                  precision floors"
type AISubdomain string

const (
	// AISubdomainUnknown is returned for non-AI signals or AI
	// signals not yet classified. Treat as "do not surface as AI"
	// rather than "treat as inventory."
	AISubdomainUnknown AISubdomain = ""

	// AISubdomainInventory covers signals derived from explicit
	// declarations or directly observable structure: which models
	// and prompts exist, which eval frameworks are configured,
	// which surfaces declare safety / capability metadata.
	// High-trust because the source data is ground truth.
	AISubdomainInventory AISubdomain = "inventory"

	// AISubdomainHygiene covers signals derived from heuristic
	// pattern matching on source code: prompt-injection structure,
	// hardcoded keys, deprecated model strings, missing sandbox
	// markers on destructive tools. Medium-trust — heuristics with
	// known false-positive patterns; honest about it.
	AISubdomainHygiene AISubdomain = "hygiene"

	// AISubdomainRegression covers signals that depend on the
	// presence of eval artifacts (Promptfoo / DeepEval / Ragas
	// outputs) to fire: cost trends, hallucination-rate changes,
	// retrieval drift, embedding-model swaps. Medium-trust because
	// the underlying eval framework's metadata is the source of
	// truth and Terrain just reads it.
	AISubdomainRegression AISubdomain = "regression"
)

// aiSubdomainBySignal maps each AI-domain SignalType to its trust
// tier. Adding a new AI signal type WITHOUT adding it here means it
// surfaces under the legacy umbrella "AI" stanza without a tier
// badge; the manifest test asserts every CategoryAI signal has a
// non-empty subdomain so this can't drift unnoticed.
var aiSubdomainBySignal = map[models.SignalType]AISubdomain{
	// ── Inventory ──────────────────────────────────────────────
	// Direct facts about declared AI surface: which prompts /
	// models / scenarios / capabilities exist. These flow from
	// `terrain ai list` and the AI surface inventory pass.
	SignalAIPolicyViolation:       AISubdomainInventory,
	SignalAIPromptVersioning:      AISubdomainInventory,
	SignalAISafetyEvalMissing:     AISubdomainInventory,
	SignalUncoveredAISurface:      AISubdomainInventory,
	SignalUntestedPromptFlow:      AISubdomainInventory,
	SignalCapabilityValidationGap: AISubdomainInventory,
	SignalPhantomEvalScenario:     AISubdomainInventory,

	// ── Hygiene ────────────────────────────────────────────────
	// Heuristic structural patterns: detector reads source code
	// and flags shapes. Medium trust; false-positive guidance per
	// detector lives in docs/rules/ai/.
	SignalAIPromptInjectionRisk:  AISubdomainHygiene,
	SignalAIHardcodedAPIKey:      AISubdomainHygiene,
	SignalAIToolWithoutSandbox:   AISubdomainHygiene,
	SignalAIModelDeprecationRisk: AISubdomainHygiene,
	SignalAIFewShotContamination: AISubdomainHygiene,
	SignalContextOverflowRisk:    AISubdomainHygiene,

	// ── Regression ─────────────────────────────────────────────
	// Eval-data-dependent: fire only when eval artifacts are
	// present. The "AI ingestion plays the role test ingestion
	// already plays for runtime data" framing.
	SignalAINonDeterministicEval: AISubdomainRegression,
	SignalAICostRegression:       AISubdomainRegression,
	SignalAIHallucinationRate:    AISubdomainRegression,
	SignalAIEmbeddingModelChange: AISubdomainRegression,
	SignalAIRetrievalRegression:  AISubdomainRegression,
	SignalWrongSourceSelected:    AISubdomainRegression,

	// All eval-output-driven detectors fire only when an eval
	// framework artifact is present (Promptfoo / DeepEval / Ragas).
	// Track these uniformly under regression so the trust posture
	// matches the data-quality lever (eval framework metadata).
	SignalAccuracyRegression:     AISubdomainRegression,
	SignalAgentFallbackTriggered: AISubdomainRegression,
	SignalAnswerGroundingFailure: AISubdomainRegression,
	SignalChunkingRegression:     AISubdomainRegression,
	SignalCitationMismatch:       AISubdomainRegression,
	SignalCitationMissing:        AISubdomainRegression,
	SignalCostRegression:         AISubdomainRegression,
	SignalEvalFailure:            AISubdomainRegression,
	SignalEvalRegression:         AISubdomainRegression,
	SignalHallucinationDetected:  AISubdomainRegression,
	SignalLatencyRegression:      AISubdomainRegression,
	SignalRerankerRegression:     AISubdomainRegression,
	SignalRetrievalMiss:          AISubdomainRegression,
	SignalSafetyFailure:          AISubdomainRegression,
	SignalSchemaParseFailure:     AISubdomainRegression,
	SignalStaleSourceRisk:        AISubdomainRegression,
	SignalToolBudgetExceeded:     AISubdomainRegression,
	SignalToolGuardrailViolation: AISubdomainRegression,
	SignalToolRoutingError:       AISubdomainRegression,
	SignalToolSelectionError:     AISubdomainRegression,
	SignalTopKRegression:         AISubdomainRegression,
}

// AISubdomainOf returns the AI subdomain classification for a signal
// type. Returns AISubdomainUnknown for signals that aren't in the
// CategoryAI domain or that haven't been classified yet.
//
// Use this from renderers (PR-comment, analyze report) to group AI
// findings into three visual sub-stanzas with distinct trust badges,
// not to filter findings — every classified signal still ships in
// the snapshot.
func AISubdomainOf(t models.SignalType) AISubdomain {
	if sub, ok := aiSubdomainBySignal[t]; ok {
		return sub
	}
	return AISubdomainUnknown
}

// AISubdomainLabel returns the user-facing section label for a
// subdomain. The label appears in PR-comment markdown and the
// terminal analyze report. Intentionally short (one or two words)
// so it fits on the same line as the badge.
func AISubdomainLabel(sub AISubdomain) string {
	switch sub {
	case AISubdomainInventory:
		return "Inventory"
	case AISubdomainHygiene:
		return "Hygiene"
	case AISubdomainRegression:
		return "Regression"
	default:
		return ""
	}
}

// AISubdomainTrustBadge returns the trust-tier badge for a subdomain.
// Used by renderers to render `[Tier 1]` / `[Tier 2]` next to the
// section header so adopters see the trust posture without having
// to consult a separate doc.
func AISubdomainTrustBadge(sub AISubdomain) string {
	switch sub {
	case AISubdomainInventory:
		return "[Tier 1]"
	case AISubdomainHygiene, AISubdomainRegression:
		return "[Tier 2]"
	default:
		return ""
	}
}
