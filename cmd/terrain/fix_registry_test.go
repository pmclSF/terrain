package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/deps"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
	"github.com/pmclSF/terrain/internal/terrainconfig"
)

// TestResolveTrustFloor locks the 0.4.0 default and its opt-outs: the
// remediation-validity gate is ON unless a user explicitly turns it off, and
// the CLI always wins over the config.
func TestResolveTrustFloor(t *testing.T) {
	ptr := func(b bool) *bool { return &b }
	cases := []struct {
		name          string
		optOn, optOff bool
		cfg           *terrainconfig.Config
		want          bool
	}{
		{"default is ON", false, false, nil, true},
		{"config unset → ON", false, false, &terrainconfig.Config{}, true},
		{"config false → OFF", false, false, &terrainconfig.Config{TrustFloor: ptr(false)}, false},
		{"config true → ON", false, false, &terrainconfig.Config{TrustFloor: ptr(true)}, true},
		{"--no-trust-floor → OFF", false, true, nil, false},
		{"--trust-floor → ON", true, false, nil, true},
		{"CLI opt-out overrides config-on", false, true, &terrainconfig.Config{TrustFloor: ptr(true)}, false},
		{"CLI opt-in overrides config-off", true, false, &terrainconfig.Config{TrustFloor: ptr(false)}, true},
		{"contradictory flags → opt-out wins", true, true, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveTrustFloor(c.optOn, c.optOff, c.cfg); got != c.want {
				t.Errorf("resolveTrustFloor(%v, %v, %+v) = %v, want %v", c.optOn, c.optOff, c.cfg, got, c.want)
			}
		})
	}
}

// TestDefaultFixRegistry_AttachesDepsPin proves the live wiring: a caret npm
// manifest flagged by drift-risk gets a structured edit_in_place Fix on its
// canonical finding via the registry the analyze path uses.
func TestDefaultFixRegistry_AttachesDepsPin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "name": "app",
  "dependencies": {"react": "^18.2.0", "lodash": "^4.17.21", "axios": "^1.6.0"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := &deps.DriftRiskDetector{Root: root}
	var fxs []findings.Finding
	for _, s := range d.Detect(nil) {
		fxs = append(fxs, findings.FromSignal(s, s.RuleID))
	}
	if len(fxs) != 1 {
		t.Fatalf("expected 1 drift-risk finding, got %d", len(fxs))
	}

	if n := defaultFixRegistry().Attach(root, fxs); n != 1 {
		t.Fatalf("Attach attached %d, want 1", n)
	}
	fix := fxs[0].Suggestions[0].Fix
	if fix == nil || fix.Kind != findings.FixEditInPlace {
		t.Errorf("expected edit_in_place Fix on the canonical finding, got %+v", fix)
	}
}

// TestTrustFloorExemptions_AlwaysGate is the blocker fix: under the default
// trust floor, a build must NEVER silently pass a definite failure, a security
// leak, a user policy violation, or a Critical finding just because Terrain has
// no auto-fix for it. Only heuristic (non-Critical, non-alwaysGate) findings are
// held back.
func TestTrustFloorExemptions_AlwaysGate(t *testing.T) {
	root := t.TempDir()
	blockable := gateBlockable(root, true) // trust floor ON
	if blockable == nil {
		t.Fatal("gateBlockable returned nil with trust floor on")
	}
	sig := func(tp models.SignalType, sev models.SignalSeverity) models.Signal {
		return models.Signal{Type: tp, Severity: sev, Category: models.CategoryAI}
	}
	cases := []struct {
		name string
		s    models.Signal
		want bool // may block CI without any validated fix?
	}{
		{"failing test blocks", sig(signals.SignalTestFailed, models.SeverityHigh), true},
		{"eval regression blocks", sig(signals.SignalEvalRegression, models.SeverityHigh), true},
		{"leaked secret blocks", sig(signals.SignalSecretsInPrompt, models.SeverityCritical), true},
		{"pii in eval blocks", sig(signals.SignalPIIInEval, models.SeverityCritical), true},
		{"user policy blocks", sig(signals.SignalPolicyViolation, models.SeverityHigh), true},
		{"ai policy blocks", sig(signals.SignalAIPolicyViolation, models.SeverityHigh), true},
		{"any Critical blocks", sig(signals.SignalAIToolWithoutSandbox, models.SeverityCritical), true},
		// A heuristic AI detector with no validated fix is HELD BACK (advisory).
		{"heuristic AI held back", sig(signals.SignalAIToolWithoutSandbox, models.SeverityHigh), false},
		{"model-deprecation held back", sig(signals.SignalAIModelDeprecationRisk, models.SeverityHigh), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := blockable(c.s); got != c.want {
				t.Errorf("gateBlockable(%s @ %s) = %v, want %v", c.s.Type, c.s.Severity, got, c.want)
			}
		})
	}
}

func detectDepsSignals(t *testing.T, body string) (string, []models.Signal) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &deps.DriftRiskDetector{Root: root}
	return root, d.Detect(nil)
}

// TestTrustFloorGateBreakdown_ExcludesUnprovenRemediation is the gate-level
// proof of the 0.4.0 goal: a high-severity finding whose remediation Terrain
// can mechanically PROVE (caret deps → pin) still blocks CI, but the same
// rule firing where the remediation is only advisable (strict-pin deps, no
// version to pin to) is excluded from the gate — surfaced, not blocking.
func TestTrustFloorGateBreakdown_ExcludesUnprovenRemediation(t *testing.T) {
	caretRoot, caretSigs := detectDepsSignals(t, `{
  "name": "app",
  "dependencies": {"react": "^18.2.0", "lodash": "^4.17.21", "axios": "^1.6.0"}
}`)
	caretBD := trustFloorGateBreakdown(caretRoot, caretSigs)
	if caretBD.Total == 0 {
		t.Fatal("caret deps carry a validated remediation and must count toward the gate")
	}

	strictRoot, strictSigs := detectDepsSignals(t, `{
  "name": "app",
  "dependencies": {"react": "*", "lodash": "latest", "axios": "*"}
}`)
	// Sanity: the strict-pin manifest does fire the same rule (so the
	// exclusion below is meaningful, not just an empty detection).
	if len(strictSigs) == 0 {
		t.Fatal("setup: strict-pin manifest should still trip drift-risk")
	}
	strictBD := trustFloorGateBreakdown(strictRoot, strictSigs)
	if strictBD.Total != 0 {
		t.Errorf("strict-pin deps remediation is judge-only and must NOT block CI; got %+v", strictBD)
	}
}
