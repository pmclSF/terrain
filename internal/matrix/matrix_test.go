package matrix

import (
	"testing"

	"github.com/pmclSF/terrain/internal/depgraph"
	"github.com/pmclSF/terrain/internal/models"
)

// buildTestSnapshotAndGraph constructs a proper snapshot and graph.
func buildTestSnapshotAndGraph() *depgraph.Graph {
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{
				Path:           "test/auth_test.go",
				Framework:      "go",
				TestCount:      3,
				EnvironmentIDs: []string{"env:ci-linux"},
				DeviceIDs:      []string{"device:chrome-120"},
			},
			{
				Path:           "test/payment_test.go",
				Framework:      "go",
				TestCount:      5,
				EnvironmentIDs: []string{"env:ci-linux", "env:ci-macos"},
				DeviceIDs:      []string{"device:chrome-120", "device:safari-17"},
			},
			{
				Path:      "test/billing_test.go",
				Framework: "go",
				TestCount: 2,
			},
		},
		Environments: []models.Environment{
			{EnvironmentID: "env:ci-linux", Name: "Linux", OS: "linux"},
			{EnvironmentID: "env:ci-macos", Name: "macOS", OS: "macos"},
			{EnvironmentID: "env:ci-windows", Name: "Windows", OS: "windows"},
		},
		EnvironmentClasses: []models.EnvironmentClass{
			{
				ClassID:   "envclass:os",
				Name:      "Operating Systems",
				Dimension: "os",
				MemberIDs: []string{"env:ci-linux", "env:ci-macos", "env:ci-windows"},
			},
			{
				ClassID:   "envclass:browser",
				Name:      "Browsers",
				Dimension: "browser",
			},
		},
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:chrome-120", Name: "Chrome 120", Platform: "web-browser", BrowserEngine: "chromium", ClassID: "envclass:browser"},
			{DeviceID: "device:safari-17", Name: "Safari 17", Platform: "web-browser", BrowserEngine: "webkit", ClassID: "envclass:browser"},
			{DeviceID: "device:firefox-121", Name: "Firefox 121", Platform: "web-browser", BrowserEngine: "gecko", ClassID: "envclass:browser"},
		},
	}

	return depgraph.Build(snap)
}

func TestAnalyze_ClassCoverage(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()
	result := Analyze(g)

	if result.ClassesAnalyzed != 2 {
		t.Errorf("expected 2 classes analyzed, got %d", result.ClassesAnalyzed)
	}
	if result.TestsAnalyzed != 3 {
		t.Errorf("expected 3 test files analyzed, got %d", result.TestsAnalyzed)
	}

	// Find OS class.
	var osClass *ClassCoverage
	var browserClass *ClassCoverage
	for i := range result.Classes {
		if result.Classes[i].ClassID == "envclass:os" {
			osClass = &result.Classes[i]
		}
		if result.Classes[i].ClassID == "envclass:browser" {
			browserClass = &result.Classes[i]
		}
	}

	if osClass == nil {
		t.Fatal("expected OS class in results")
	}
	if osClass.TotalMembers != 3 {
		t.Errorf("expected 3 OS members, got %d", osClass.TotalMembers)
	}
	// Linux and macOS are targeted, Windows is not.
	if osClass.CoveredMembers != 2 {
		t.Errorf("expected 2 covered OS members, got %d", osClass.CoveredMembers)
	}

	if browserClass == nil {
		t.Fatal("expected browser class in results")
	}
	if browserClass.TotalMembers != 3 {
		t.Errorf("expected 3 browser members, got %d", browserClass.TotalMembers)
	}
	// Chrome and Safari are targeted, Firefox is not.
	if browserClass.CoveredMembers != 2 {
		t.Errorf("expected 2 covered browser members, got %d", browserClass.CoveredMembers)
	}
}

func TestAnalyze_Gaps(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()
	result := Analyze(g)

	if len(result.Gaps) != 2 {
		t.Fatalf("expected 2 gaps (Windows + Firefox), got %d", len(result.Gaps))
	}

	gapIDs := map[string]bool{}
	for _, gap := range result.Gaps {
		gapIDs[gap.MemberID] = true
	}
	if !gapIDs["env:ci-windows"] {
		t.Error("expected Windows gap")
	}
	if !gapIDs["device:firefox-121"] {
		t.Error("expected Firefox gap")
	}
}

func TestAnalyze_Concentration(t *testing.T) {
	t.Parallel()
	// Build a graph where one OS dominates.
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.go", Framework: "go", TestCount: 1, EnvironmentIDs: []string{"env:linux"}},
			{Path: "test/b.go", Framework: "go", TestCount: 1, EnvironmentIDs: []string{"env:linux"}},
			{Path: "test/c.go", Framework: "go", TestCount: 1, EnvironmentIDs: []string{"env:linux"}},
			{Path: "test/d.go", Framework: "go", TestCount: 1, EnvironmentIDs: []string{"env:linux"}},
			// Only one test on macOS.
		},
		Environments: []models.Environment{
			{EnvironmentID: "env:linux", Name: "Linux"},
			{EnvironmentID: "env:macos", Name: "macOS"},
		},
		EnvironmentClasses: []models.EnvironmentClass{
			{
				ClassID:   "envclass:os",
				Name:      "OS",
				Dimension: "os",
				MemberIDs: []string{"env:linux", "env:macos"},
			},
		},
	}

	g := depgraph.Build(snap)
	result := Analyze(g)

	if len(result.Concentrations) != 1 {
		t.Fatalf("expected 1 concentration, got %d", len(result.Concentrations))
	}
	conc := result.Concentrations[0]
	if conc.DominantMember != "env:linux" {
		t.Errorf("expected dominant member env:linux, got %s", conc.DominantMember)
	}
	if conc.DominantShare < 0.99 {
		t.Errorf("expected ~100%% dominant share, got %.2f", conc.DominantShare)
	}
}

func TestAnalyze_Recommendations(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()
	result := Analyze(g)

	if len(result.Recommendations) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	// Should recommend Windows and Firefox as they are uncovered.
	recIDs := map[string]bool{}
	for _, rec := range result.Recommendations {
		recIDs[rec.MemberID] = true
		if rec.Reason == "" {
			t.Errorf("recommendation for %s has empty reason", rec.MemberID)
		}
		if rec.Priority == 0 {
			t.Errorf("recommendation for %s has zero priority", rec.MemberID)
		}
	}
	if !recIDs["env:ci-windows"] {
		t.Error("expected recommendation for Windows")
	}
	if !recIDs["device:firefox-121"] {
		t.Error("expected recommendation for Firefox")
	}
}

func TestAnalyze_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := depgraph.NewGraph()
	result := Analyze(g)

	if len(result.Classes) != 0 {
		t.Errorf("expected 0 classes, got %d", len(result.Classes))
	}
	if len(result.Gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(result.Gaps))
	}
	if len(result.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(result.Recommendations))
	}
}

func TestAnalyze_NilGraph(t *testing.T) {
	t.Parallel()
	result := Analyze(nil)

	if result == nil {
		t.Fatal("expected non-nil result for nil graph")
	}
	if len(result.Classes) != 0 {
		t.Errorf("expected 0 classes, got %d", len(result.Classes))
	}
}

func TestAnalyze_NoGapsWhenFullCoverage(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.go", Framework: "go", TestCount: 1, EnvironmentIDs: []string{"env:linux", "env:macos"}},
		},
		Environments: []models.Environment{
			{EnvironmentID: "env:linux", Name: "Linux"},
			{EnvironmentID: "env:macos", Name: "macOS"},
		},
		EnvironmentClasses: []models.EnvironmentClass{
			{ClassID: "envclass:os", Name: "OS", Dimension: "os", MemberIDs: []string{"env:linux", "env:macos"}},
		},
	}

	g := depgraph.Build(snap)
	result := Analyze(g)

	if len(result.Gaps) != 0 {
		t.Errorf("expected 0 gaps with full coverage, got %d", len(result.Gaps))
	}
	if len(result.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations with full coverage, got %d", len(result.Recommendations))
	}
}

func TestAnalyze_ClassWithNoCoveredMembers(t *testing.T) {
	t.Parallel()
	// A class exists but no tests target any member.
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.go", Framework: "go", TestCount: 1},
		},
		Environments: []models.Environment{
			{EnvironmentID: "env:linux", Name: "Linux"},
		},
		EnvironmentClasses: []models.EnvironmentClass{
			{ClassID: "envclass:os", Name: "OS", Dimension: "os", MemberIDs: []string{"env:linux"}},
		},
	}

	g := depgraph.Build(snap)
	result := Analyze(g)

	// Gaps exist but recommendations should NOT include them since the class
	// has zero covered members (may not be relevant to the project).
	if len(result.Gaps) != 1 {
		t.Errorf("expected 1 gap, got %d", len(result.Gaps))
	}
	if len(result.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations for class with no covered members, got %d", len(result.Recommendations))
	}
}

func TestAnalyze_Deterministic(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()

	r1 := Analyze(g)
	r2 := Analyze(g)

	if len(r1.Classes) != len(r2.Classes) {
		t.Fatalf("non-deterministic class count: %d vs %d", len(r1.Classes), len(r2.Classes))
	}
	for i := range r1.Classes {
		if r1.Classes[i].ClassID != r2.Classes[i].ClassID {
			t.Errorf("non-deterministic class ID at %d: %s vs %s",
				i, r1.Classes[i].ClassID, r2.Classes[i].ClassID)
		}
	}
	if len(r1.Recommendations) != len(r2.Recommendations) {
		t.Fatalf("non-deterministic recommendation count: %d vs %d",
			len(r1.Recommendations), len(r2.Recommendations))
	}
	for i := range r1.Recommendations {
		if r1.Recommendations[i].MemberID != r2.Recommendations[i].MemberID {
			t.Errorf("non-deterministic recommendation at %d", i)
		}
	}
}

func TestRecommendForTests_RelevantClasses(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()

	// Recommend for auth test which targets Linux and Chrome.
	recs := RecommendForTests(g, []string{"test/auth_test.go"})

	if len(recs) == 0 {
		t.Fatal("expected recommendations for auth test")
	}

	// Should recommend macOS and Windows (OS class members not targeted),
	// and Safari and Firefox (browser class members not targeted).
	recIDs := map[string]bool{}
	for _, rec := range recs {
		recIDs[rec.MemberID] = true
	}

	// OS class: auth targets Linux, so macOS and Windows should be recommended.
	if !recIDs["env:ci-macos"] {
		t.Error("expected recommendation for macOS")
	}
	if !recIDs["env:ci-windows"] {
		t.Error("expected recommendation for Windows")
	}
	// Browser class: auth targets Chrome, so Safari and Firefox should be recommended.
	if !recIDs["device:safari-17"] {
		t.Error("expected recommendation for Safari")
	}
	if !recIDs["device:firefox-121"] {
		t.Error("expected recommendation for Firefox")
	}
}

func TestRecommendForTests_Empty(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()

	recs := RecommendForTests(g, nil)
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations for empty test list, got %d", len(recs))
	}

	recs = RecommendForTests(nil, []string{"test/a.go"})
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations for nil graph, got %d", len(recs))
	}
}

func TestRecommendForTests_IrrelevantTests(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()

	// billing_test.go has no environment targets — no class is relevant.
	recs := RecommendForTests(g, []string{"test/billing_test.go"})
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations for test with no env targets, got %d", len(recs))
	}
}

func TestFormatSummary_NonEmpty(t *testing.T) {
	t.Parallel()
	g := buildTestSnapshotAndGraph()
	result := Analyze(g)

	summary := FormatSummary(result)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !contains(summary, "Matrix Coverage") {
		t.Error("expected 'Matrix Coverage' in summary")
	}
	if !contains(summary, "Gaps") {
		t.Error("expected 'Gaps' in summary")
	}
}

func TestFormatSummary_Empty(t *testing.T) {
	t.Parallel()
	result := Analyze(nil)
	summary := FormatSummary(result)
	if !contains(summary, "not applicable") {
		t.Error("expected 'not applicable' in empty summary")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Mobile device matrix E2E integration tests
// ---------------------------------------------------------------------------

// buildMobileDeviceMatrix constructs a realistic mobile test scenario:
// tests targeting iOS and Android devices with coverage gaps.
func buildMobileDeviceMatrix() *depgraph.Graph {
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{
				Path:      "tests/e2e/login.test.ts",
				Framework: "playwright",
				TestCount: 4,
				DeviceIDs: []string{"device:iphone-15-ios17", "device:pixel-8-android14"},
			},
			{
				Path:      "tests/e2e/purchase.test.ts",
				Framework: "playwright",
				TestCount: 6,
				DeviceIDs: []string{"device:iphone-15-ios17"},
			},
			{
				Path:      "tests/e2e/checkout.test.ts",
				Framework: "playwright",
				TestCount: 3,
				DeviceIDs: []string{"device:iphone-15-ios17"},
			},
		},
		DeviceConfigs: []models.DeviceConfig{
			{DeviceID: "device:iphone-15-ios17", Name: "iPhone 15", Platform: "ios", FormFactor: "phone", OSVersion: "17.0", ClassID: "envclass:mobile-device"},
			{DeviceID: "device:pixel-8-android14", Name: "Pixel 8", Platform: "android", FormFactor: "phone", OSVersion: "14", ClassID: "envclass:mobile-device"},
			{DeviceID: "device:ipad-pro-ios17", Name: "iPad Pro", Platform: "ios", FormFactor: "tablet", OSVersion: "17.0", ClassID: "envclass:mobile-device"},
			{DeviceID: "device:galaxy-s24-android14", Name: "Galaxy S24", Platform: "android", FormFactor: "phone", OSVersion: "14", ClassID: "envclass:mobile-device"},
		},
		EnvironmentClasses: []models.EnvironmentClass{
			{
				ClassID:   "envclass:mobile-device",
				Name:      "Mobile Devices",
				Dimension: "device",
				MemberIDs: []string{
					"device:iphone-15-ios17",
					"device:pixel-8-android14",
					"device:ipad-pro-ios17",
					"device:galaxy-s24-android14",
				},
			},
		},
	}
	return depgraph.Build(snap)
}

func TestAnalyze_MobileDeviceMatrix_DetectsGaps(t *testing.T) {
	t.Parallel()
	g := buildMobileDeviceMatrix()
	result := Analyze(g)

	if result.ClassesAnalyzed != 1 {
		t.Fatalf("expected 1 class (mobile-device), got %d", result.ClassesAnalyzed)
	}

	// 2 of 4 devices have no test coverage (iPad Pro, Galaxy S24).
	if len(result.Gaps) != 2 {
		t.Fatalf("expected 2 gaps (iPad Pro, Galaxy S24), got %d", len(result.Gaps))
	}

	gapNames := map[string]bool{}
	for _, gap := range result.Gaps {
		gapNames[gap.MemberName] = true
	}
	if !gapNames["iPad Pro"] {
		t.Error("expected iPad Pro in gaps")
	}
	if !gapNames["Galaxy S24"] {
		t.Error("expected Galaxy S24 in gaps")
	}
}

func TestAnalyze_MobileDeviceMatrix_DetectsConcentration(t *testing.T) {
	t.Parallel()
	g := buildMobileDeviceMatrix()
	result := Analyze(g)

	// iPhone 15 is targeted by 3/3 test files; Pixel 8 by only 1/3.
	// iPhone 15 share = 3 / (3+1) = 75% > 70% threshold.
	// And 2 of 4 members uncovered, so concentration should fire.
	if len(result.Concentrations) != 1 {
		t.Fatalf("expected 1 concentration, got %d", len(result.Concentrations))
	}
	conc := result.Concentrations[0]
	if conc.DominantName != "iPhone 15" {
		t.Errorf("dominant = %q, want 'iPhone 15'", conc.DominantName)
	}
	if conc.DominantShare < 0.70 {
		t.Errorf("dominant share = %.2f, want > 0.70", conc.DominantShare)
	}
}

func TestAnalyze_MobileDeviceMatrix_ProducesRecommendations(t *testing.T) {
	t.Parallel()
	g := buildMobileDeviceMatrix()
	result := Analyze(g)

	if len(result.Recommendations) == 0 {
		t.Fatal("expected recommendations for uncovered mobile devices")
	}

	recNames := map[string]bool{}
	for _, rec := range result.Recommendations {
		recNames[rec.MemberName] = true
		if rec.Priority < 1 {
			t.Errorf("recommendation %q has invalid priority %d", rec.MemberName, rec.Priority)
		}
		if rec.Reason == "" {
			t.Errorf("recommendation %q has empty reason", rec.MemberName)
		}
	}

	// iPad Pro and Galaxy S24 should be recommended.
	if !recNames["iPad Pro"] {
		t.Error("expected iPad Pro in recommendations")
	}
	if !recNames["Galaxy S24"] {
		t.Error("expected Galaxy S24 in recommendations")
	}
}

func TestAnalyze_MobileDeviceMatrix_CoverageRatio(t *testing.T) {
	t.Parallel()
	g := buildMobileDeviceMatrix()
	result := Analyze(g)

	if len(result.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(result.Classes))
	}
	cc := result.Classes[0]
	if cc.TotalMembers != 4 {
		t.Errorf("total members = %d, want 4", cc.TotalMembers)
	}
	if cc.CoveredMembers != 2 {
		t.Errorf("covered members = %d, want 2", cc.CoveredMembers)
	}
	// 2/4 = 0.50
	if cc.CoverageRatio != 0.5 {
		t.Errorf("coverage ratio = %.2f, want 0.50", cc.CoverageRatio)
	}
}

func TestFormatSummary_MobileMatrix(t *testing.T) {
	t.Parallel()
	g := buildMobileDeviceMatrix()
	result := Analyze(g)
	summary := FormatSummary(result)

	if !contains(summary, "Mobile Devices") {
		t.Error("expected 'Mobile Devices' in summary")
	}
	if !contains(summary, "2/4 members covered") {
		t.Error("expected '2/4 members covered' in summary")
	}
	if !contains(summary, "Gaps") {
		t.Error("expected 'Gaps' in summary")
	}
	if !contains(summary, "iPad Pro") {
		t.Error("expected 'iPad Pro' in gap list")
	}
}
