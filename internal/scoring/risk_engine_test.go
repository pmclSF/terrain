package scoring

import (
	"runtime"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestComputeRisk_NoSignals(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{}
	surfaces := ComputeRisk(snap)
	if len(surfaces) != 0 {
		t.Errorf("expected 0 risk surfaces, got %d", len(surfaces))
	}
}

func TestComputeRisk_ReliabilitySignals(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "flakyTest", Severity: models.SeverityHigh},
			{Type: "skippedTest", Severity: models.SeverityLow},
		},
	}
	surfaces := ComputeRisk(snap)

	var found bool
	for _, s := range surfaces {
		if s.Type == "reliability" && s.Scope == "repository" {
			found = true
			// high=3 + low=1 = 4 → medium band
			if s.Band != models.RiskBandMedium {
				t.Errorf("band = %q, want medium (score=%.1f)", s.Band, s.Score)
			}
			if s.Score != 4.0 {
				t.Errorf("score = %.1f, want 4.0", s.Score)
			}
		}
	}
	if !found {
		t.Error("expected a reliability risk surface")
	}
}

func TestComputeRisk_ChangeRiskSignals(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", Severity: models.SeverityMedium},
			{Type: "untestedExport", Severity: models.SeverityMedium},
		},
	}
	surfaces := ComputeRisk(snap)

	var found bool
	for _, s := range surfaces {
		if s.Type == "change" && s.Scope == "repository" {
			found = true
			// medium=2 + medium=2 = 4 → medium band
			if s.Band != models.RiskBandMedium {
				t.Errorf("band = %q, want medium", s.Band)
			}
		}
	}
	if !found {
		t.Error("expected a change risk surface")
	}
}

func TestComputeRisk_GovernanceSignals(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "policyViolation", Severity: models.SeverityHigh},
		},
	}
	surfaces := ComputeRisk(snap)

	var found bool
	for _, s := range surfaces {
		if s.Type == "governance" && s.Scope == "repository" {
			found = true
			if s.Band == models.RiskBandLow {
				t.Errorf("expected governance risk to be at least medium for a high-severity policy violation, got %q", s.Band)
			}
		}
	}
	if !found {
		t.Error("expected a governance risk surface")
	}
}

func TestComputeRisk_LargeRepoAbsoluteBurden(t *testing.T) {
	t.Parallel()
	testFiles := make([]models.TestFile, 5000)
	signals := make([]models.Signal, 10)
	for i := range signals {
		signals[i] = models.Signal{Type: "weakAssertion", Severity: models.SeverityCritical}
	}

	snap := &models.TestSuiteSnapshot{
		TestFiles: testFiles,
		Signals:   signals,
	}
	surfaces := ComputeRisk(snap)

	var found bool
	for _, s := range surfaces {
		if s.Type == "change" && s.Scope == "repository" {
			found = true
			if s.Band == models.RiskBandLow {
				t.Fatalf("expected non-low change risk with 10 critical findings, got low (score=%.2f)", s.Score)
			}
		}
	}
	if !found {
		t.Fatal("expected repository change risk surface")
	}
}

func TestComputeRisk_DirectoryRollup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory rollup map keyed by filepath.Dir output without ToSlash; tracked in #114")
	}
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		Signals: []models.Signal{
			{Type: "weakAssertion", Severity: models.SeverityMedium, Location: models.SignalLocation{File: "src/auth/login.test.js"}},
			{Type: "untestedExport", Severity: models.SeverityHigh, Location: models.SignalLocation{File: "src/auth/signup.js"}},
		},
	}
	surfaces := ComputeRisk(snap)

	var dirFound bool
	for _, s := range surfaces {
		if s.Type == "change" && s.Scope == "directory" && s.ScopeName == "src/auth" {
			dirFound = true
		}
	}
	if !dirFound {
		t.Error("expected a directory-level change risk surface for src/auth")
	}
}

func TestComputeRisk_HysteresisHoldsLowUntilThreshold(t *testing.T) {
	t.Parallel()
	testFiles := make([]models.TestFile, 7)
	snap := &models.TestSuiteSnapshot{
		TestFiles: testFiles,
		Signals: []models.Signal{
			{Type: "weakAssertion", Severity: models.SeverityHigh}, // score ~4.29 without previous band
		},
		Risk: []models.RiskSurface{
			{Type: "change", Scope: "repository", Band: models.RiskBandLow},
		},
	}

	surfaces := ComputeRisk(snap)
	for _, s := range surfaces {
		if s.Type == "change" && s.Scope == "repository" {
			if s.Band != models.RiskBandLow {
				t.Fatalf("expected hysteresis to keep low band near threshold, got %q (score=%.2f)", s.Band, s.Score)
			}
			return
		}
	}
	t.Fatal("expected repository change surface")
}

func TestComputeRisk_HysteresisAvoidsMediumDrop(t *testing.T) {
	t.Parallel()
	testFiles := make([]models.TestFile, 8)
	snap := &models.TestSuiteSnapshot{
		TestFiles: testFiles,
		Signals: []models.Signal{
			{Type: "weakAssertion", Severity: models.SeverityMedium},
			{Type: "mockHeavyTest", Severity: models.SeverityLow}, // score ~3.75
		},
		Risk: []models.RiskSurface{
			{Type: "change", Scope: "repository", Band: models.RiskBandMedium},
		},
	}

	surfaces := ComputeRisk(snap)
	for _, s := range surfaces {
		if s.Type == "change" && s.Scope == "repository" {
			if s.Band != models.RiskBandMedium {
				t.Fatalf("expected hysteresis to keep medium band near threshold, got %q (score=%.2f)", s.Band, s.Score)
			}
			return
		}
	}
	t.Fatal("expected repository change surface")
}

func TestScoreToBand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		score float64
		want  models.RiskBand
	}{
		{0, models.RiskBandLow},
		{3, models.RiskBandLow},
		{4, models.RiskBandMedium},
		{8, models.RiskBandMedium},
		{9, models.RiskBandHigh},
		{15, models.RiskBandHigh},
		{16, models.RiskBandCritical},
		{100, models.RiskBandCritical},
	}
	for _, tt := range tests {
		got := scoreToBand(tt.score)
		if got != tt.want {
			t.Errorf("scoreToBand(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

// TestScoreToBand_Boundaries pins the exact band assignment at every
// threshold. Round 4 review identified that small score perturbations could
// silently flip bands without anyone noticing; this test acts as a tripwire.
//
// If you intentionally change a band threshold, update the rubric in
// docs/scoring-rubric.md AND update the matching named constant in
// risk_engine.go AND adjust this table — failing in only one place is what
// drift looks like.
func TestScoreToBand_Boundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score float64
		want  models.RiskBand
	}{
		// Just below each boundary stays in the lower band.
		{3.99, models.RiskBandLow},
		{8.99, models.RiskBandMedium},
		{15.99, models.RiskBandHigh},

		// Exactly on each boundary jumps to the upper band (>=, not >).
		{4.00, models.RiskBandMedium},
		{9.00, models.RiskBandHigh},
		{16.00, models.RiskBandCritical},

		// Just above each boundary stays in the upper band.
		{4.01, models.RiskBandMedium},
		{9.01, models.RiskBandHigh},
		{16.01, models.RiskBandCritical},

		// Extremes.
		{0.00, models.RiskBandLow},
		{0.01, models.RiskBandLow},
		{1000.00, models.RiskBandCritical},
	}

	for _, tc := range cases {
		got := scoreToBand(tc.score)
		if got != tc.want {
			t.Errorf("scoreToBand(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

// TestScoreToBandWithHysteresis_DoesNotFlap ensures the deadband prevents
// a one-step flip when a previous band is established. We pick scores
// inside each hysteresis window and confirm the previous band is held.
func TestScoreToBandWithHysteresis_DoesNotFlap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		previous models.RiskBand
		score    float64
		want     models.RiskBand
	}{
		// Coming from Low, a score of 4.0 normally bumps to Medium, but with
		// previousBand=Low and lowUp = 4 + 0.5 = 4.5, we hold at Low.
		{"Low holds in 4.0–4.5 band", models.RiskBandLow, 4.2, models.RiskBandLow},

		// Coming from Medium, a score of 9.0 normally bumps to High, but with
		// previousBand=Medium we need >= 9.5 to actually flip.
		{"Medium holds in 9.0–9.5 band", models.RiskBandMedium, 9.2, models.RiskBandMedium},

		// And in the other direction: Medium score 3.6 normally drops to Low,
		// but Medium → Low needs < 3.5.
		{"Medium holds in 3.5–4.0 band", models.RiskBandMedium, 3.6, models.RiskBandMedium},

		// Cross the deadband fully and the flip is allowed.
		{"Medium drops past hysteresis", models.RiskBandMedium, 3.4, models.RiskBandLow},

		// First-run (no previous band) uses plain mapping; 4.2 → Medium.
		{"First-run uses scoreToBand", "", 4.2, models.RiskBandMedium},
	}

	for _, tc := range cases {
		got := scoreToBandWithHysteresis(tc.score, tc.previous)
		if got != tc.want {
			t.Errorf("%s: scoreToBandWithHysteresis(%v, %q) = %q, want %q",
				tc.name, tc.score, tc.previous, got, tc.want)
		}
	}
}

// TestSeverityWeights_Monotonic guards against a future edit accidentally
// reversing the severity ordering (e.g. someone swapping High and Medium
// while editing). Round 4 noted these weights as a frequent target for
// drift; this test pins the contract.
func TestSeverityWeights_Monotonic(t *testing.T) {
	t.Parallel()

	if !(severityWeight[models.SeverityCritical] >
		severityWeight[models.SeverityHigh] &&
		severityWeight[models.SeverityHigh] >
			severityWeight[models.SeverityMedium] &&
		severityWeight[models.SeverityMedium] >
			severityWeight[models.SeverityLow] &&
		severityWeight[models.SeverityLow] >
			severityWeight[models.SeverityInfo]) {
		t.Errorf("severity weights are not strictly decreasing: critical=%v high=%v medium=%v low=%v info=%v",
			severityWeight[models.SeverityCritical],
			severityWeight[models.SeverityHigh],
			severityWeight[models.SeverityMedium],
			severityWeight[models.SeverityLow],
			severityWeight[models.SeverityInfo],
		)
	}
}
