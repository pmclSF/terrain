package signals

import (
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

// TestAISubdomain_AllAISignalsClassified asserts every CategoryAI
// signal in the manifest has a non-empty subdomain mapping. This is
// the drift gate: adding a new AI signal type without classifying it
// means the renderer will silently lump it into the legacy umbrella
// stanza, betraying the trust-tier UX adopters depend on.
func TestAISubdomain_AllAISignalsClassified(t *testing.T) {
	t.Parallel()
	for _, entry := range Manifest() {
		if entry.Domain != models.CategoryAI {
			continue
		}
		sub := AISubdomainOf(entry.Type)
		if sub == AISubdomainUnknown {
			t.Errorf("AI signal %q has no subdomain classification — add it to aiSubdomainBySignal in ai_subdomain.go",
				entry.Type)
		}
	}
}

func TestAISubdomain_KnownSamples(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		t    models.SignalType
		want AISubdomain
	}{
		{"hygiene/promptInjection", SignalAIPromptInjectionRisk, AISubdomainHygiene},
		{"hygiene/hardcodedKey", SignalAIHardcodedAPIKey, AISubdomainHygiene},
		{"hygiene/modelDeprecation", SignalAIModelDeprecationRisk, AISubdomainHygiene},
		{"regression/cost", SignalAICostRegression, AISubdomainRegression},
		{"regression/hallucination", SignalAIHallucinationRate, AISubdomainRegression},
		{"regression/retrieval", SignalAIRetrievalRegression, AISubdomainRegression},
		{"inventory/promptVersioning", SignalAIPromptVersioning, AISubdomainInventory},
		{"inventory/safetyEval", SignalAISafetyEvalMissing, AISubdomainInventory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AISubdomainOf(tt.t)
			if got != tt.want {
				t.Errorf("AISubdomainOf(%q) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

func TestAISubdomain_NonAISignalReturnsUnknown(t *testing.T) {
	t.Parallel()
	got := AISubdomainOf(SignalSlowTest)
	if got != AISubdomainUnknown {
		t.Errorf("AISubdomainOf(non-AI) = %q, want unknown", got)
	}
}

func TestAISubdomainLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sub  AISubdomain
		want string
	}{
		{AISubdomainInventory, "Inventory"},
		{AISubdomainHygiene, "Hygiene"},
		{AISubdomainRegression, "Regression"},
		{AISubdomainUnknown, ""},
	}
	for _, tt := range tests {
		if got := AISubdomainLabel(tt.sub); got != tt.want {
			t.Errorf("Label(%q) = %q, want %q", tt.sub, got, tt.want)
		}
	}
}

func TestAISubdomainTrustBadge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sub  AISubdomain
		want string
	}{
		{AISubdomainInventory, "[Tier 1]"},
		{AISubdomainHygiene, "[Tier 2]"},
		{AISubdomainRegression, "[Tier 2]"},
		{AISubdomainUnknown, ""},
	}
	for _, tt := range tests {
		if got := AISubdomainTrustBadge(tt.sub); got != tt.want {
			t.Errorf("Badge(%q) = %q, want %q", tt.sub, got, tt.want)
		}
	}
}
