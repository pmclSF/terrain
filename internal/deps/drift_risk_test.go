package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestDriftRiskDetector_NPMHighDrift(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
	"dependencies": {
		"left-pad":  "^1.0.0",
		"is-odd":    "*",
		"chalk":     "latest",
		"react":     "18.2.0"
	}
}`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	sig := out[0]
	if sig.Type != signals.SignalDepsDriftRisk {
		t.Errorf("unexpected signal type: %v", sig.Type)
	}
	if sig.Category != models.CategoryQuality {
		t.Errorf("unexpected category: %v", sig.Category)
	}
	share, _ := sig.Metadata["movingTargetShare"].(float64)
	if share < 0.40 {
		t.Errorf("expected share >= 0.40, got %v", share)
	}
	if eco, _ := sig.Metadata["ecosystem"].(string); eco != "npm" {
		t.Errorf("unexpected ecosystem: %v", eco)
	}
}

func TestDriftRiskDetector_NPMPinnedNoFinding(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
	"dependencies": {
		"left-pad":  "1.0.0",
		"is-odd":    "2.0.0",
		"chalk":     "5.3.0",
		"react":     "18.2.0"
	}
}`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 0 {
		t.Fatalf("expected no findings for fully pinned manifest, got %d", len(out))
	}
}

func TestDriftRiskDetector_PipRequirements(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "requirements.txt"), `# top comment
requests
flask>=2.0
django==4.2.1
numpy<2.0
pandas
`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	if eco, _ := out[0].Metadata["ecosystem"].(string); eco != "pip" {
		t.Errorf("unexpected ecosystem: %v", eco)
	}
}

func TestDriftRiskDetector_GoModPinned(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), `module example.com/foo

go 1.21

require (
	github.com/a/b v1.2.3
	github.com/c/d v2.0.0
	github.com/e/f v1.0.0
)
`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 0 {
		t.Fatalf("expected no finding for tagged go.mod, got %d findings", len(out))
	}
}

func TestDriftRiskDetector_GoModPseudoVersions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), `module example.com/foo

go 1.21

require (
	github.com/a/b v0.0.0-20240101120000-abcdef123456
	github.com/c/d v0.0.0-20240202120000-fedcba654321
	github.com/e/f v0.0.0-20240303120000-aabbccddeeff
)
`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding for all-pseudo go.mod, got %d findings", len(out))
	}
}

func TestDriftRiskDetector_CargoMixed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Cargo.toml"), `[package]
name = "foo"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = "1.32"
log = "=0.4.20"
once_cell = "1.18"
`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding for cargo with 3/4 floating, got %d", len(out))
	}
}

func TestDriftRiskDetector_BelowMinDeps(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
	"dependencies": {
		"left-pad": "*",
		"chalk":    "latest"
	}
}`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 0 {
		t.Fatalf("expected no finding below minDepsForFinding, got %d", len(out))
	}
}

func TestDriftRiskDetector_SkipsVendoredDirs(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"node_modules", "vendor", ".git", "target", "dist", "build", "third_party"} {
		writeFile(t, filepath.Join(root, dir, "package.json"), `{
	"dependencies": {"a":"*","b":"*","c":"*","d":"*"}
}`)
	}
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 0 {
		t.Fatalf("expected no findings under vendored dirs, got %d", len(out))
	}
}

func TestDriftRiskDetector_SeverityScaling(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
	"dependencies": {
		"a": "*", "b": "*", "c": "*", "d": "*", "e": "*",
		"f": "*", "g": "*", "h": "*", "i": "1.2.3", "j": "1.2.3"
	}
}`)
	d := &DriftRiskDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	if out[0].Severity != models.SeverityHigh {
		t.Errorf("expected High severity for >75%% share, got %v", out[0].Severity)
	}
}

func TestDriftRiskDetector_NilSafe(t *testing.T) {
	var d *DriftRiskDetector
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Errorf("expected nil from nil receiver, got %v", got)
	}
	d2 := &DriftRiskDetector{Root: ""}
	if got := d2.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Errorf("expected nil with empty Root, got %v", got)
	}
}

// TestDriftRiskDetector_SplitMechanism_On asserts the
// deps_drift_risk_split mechanism observably swaps the emitted
// signal Type between strict-pin and caret-policy based on which
// moving-target class dominates the manifest.
func TestDriftRiskDetector_SplitMechanism_On(t *testing.T) {
	// Caret-dominated manifest.
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
	"dependencies": {
		"react": "^18.0.0",
		"lodash": "^4.17.0",
		"chalk": "^5.3.0",
		"axios": "^1.6.0"
	}
}`)

	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	prev := mechanisms.SetDefault(reg)
	defer mechanisms.SetDefault(prev)

	d := &DriftRiskDetector{Root: root}

	// Shadow → legacy "depsDriftRisk" type.
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 || out[0].Type != signals.SignalDepsDriftRisk {
		t.Fatalf("shadow: expected legacy depsDriftRisk; got %v", out)
	}

	// On + caret-dominated → caret-policy split type.
	if err := reg.ApplyCLIOverrides([]string{"deps_drift_risk_split=on"}); err != nil {
		t.Fatalf("override: %v", err)
	}
	out = d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 || out[0].Type != signals.SignalDepsDriftRiskCaretPolicy {
		t.Fatalf("on + caret-dominated: expected SignalDepsDriftRiskCaretPolicy; got %v", out)
	}

	// Switch to strict-pin-dominated manifest.
	root2 := t.TempDir()
	writeFile(t, filepath.Join(root2, "package.json"), `{
	"dependencies": {
		"left-pad": "*",
		"is-odd": "*",
		"chalk": "latest",
		"react": "18.2.0"
	}
}`)
	d2 := &DriftRiskDetector{Root: root2}
	out = d2.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 || out[0].Type != signals.SignalDepsDriftRiskStrictPin {
		t.Fatalf("on + strict-pin-dominated: expected SignalDepsDriftRiskStrictPin; got %v", out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// depDetect runs the detector against a single manifest in a temp repo root.
func depDetect(t *testing.T, manifest, content string) []models.Signal {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, manifest), content)
	return (&DriftRiskDetector{Root: root}).Detect(&models.TestSuiteSnapshot{})
}

func soleFinding(t *testing.T, sigs []models.Signal) models.Signal {
	t.Helper()
	if len(sigs) != 1 {
		t.Fatalf("want exactly 1 finding, got %d: %+v", len(sigs), sigs)
	}
	return sigs[0]
}

func shareOf(t *testing.T, sig models.Signal) float64 {
	t.Helper()
	s, _ := sig.Metadata["movingTargetShare"].(float64)
	return s
}

// D5: pyproject.toml (Poetry tuple style) — previously an untested ecosystem.
func TestDriftRiskDetector_PyprojectToml(t *testing.T) {
	sig := soleFinding(t, depDetect(t, "pyproject.toml", `[dependencies]
requests = ">=2.28.0"
django = "*"
pandas = "~=2.0"
numpy = ">1.0"
flask = "==2.3.0"
`))
	if eco, _ := sig.Metadata["ecosystem"].(string); eco != "pyproject" {
		t.Errorf("ecosystem: want pyproject, got %q", eco)
	}
	if got := shareOf(t, sig); got != 0.80 {
		t.Errorf("share: want exactly 0.80 (4 moving / 5), got %v", got)
	}
	if sig.Severity != models.SeverityHigh { // share > 0.75 → High
		t.Errorf("severity: want High, got %v", sig.Severity)
	}
}

// D8: Gemfile — previously an untested ecosystem.
func TestDriftRiskDetector_Gemfile(t *testing.T) {
	sig := soleFinding(t, depDetect(t, "Gemfile", `gem "rails", "~> 7.0"
gem "puma"
gem "sqlite3", "= 1.6.0"
gem "sass-rails"
`))
	if eco, _ := sig.Metadata["ecosystem"].(string); eco != "gemfile" {
		t.Errorf("ecosystem: want gemfile, got %q", eco)
	}
	if got := shareOf(t, sig); got != 0.75 {
		t.Errorf("share: want exactly 0.75 (3 moving / 4), got %v", got)
	}
}

// D3: npm tilde (~1.2.3) is pinned-enough — not a moving target.
func TestDriftRiskDetector_NPMTildeNotMoving(t *testing.T) {
	out := depDetect(t, "package.json", `{"dependencies":{"a":"~1.2.3","b":"1.0.0","c":"2.0.0","d":"3.0.0"}}`)
	if len(out) != 0 {
		t.Errorf("tilde + exact pins → 0%% moving → no finding; got %d: %+v", len(out), out)
	}
}

// D3: npm VCS/protocol specs (git+, file:, workspace:) are moving targets.
func TestDriftRiskDetector_NPMVCSSpecs(t *testing.T) {
	sig := soleFinding(t, depDetect(t, "package.json",
		`{"dependencies":{"a":"git+https://github.com/foo/bar.git","b":"file:../local","c":"workspace:*","e":"link:../pkg","d":"1.0.0"}}`))
	if got := shareOf(t, sig); got != 0.80 {
		t.Errorf("share: want 0.80 (4 VCS/proto of 5), got %v", got)
	}
	if sig.Severity != models.SeverityHigh { // share > 0.75 → High
		t.Errorf("severity: want High, got %v", sig.Severity)
	}
}

// D2: severity scales by share — Low for 40–50%, Medium for 50–75%. The
// existing severity test only covered the High (>75%) band.
func TestDriftRiskDetector_SeverityLowAndMediumRanges(t *testing.T) {
	low := soleFinding(t, depDetect(t, "package.json",
		`{"dependencies":{"a":"*","b":"*","c":"1.0.0","d":"2.0.0","e":"3.0.0"}}`)) // 2/5 = 0.40
	if got := shareOf(t, low); got != 0.40 {
		t.Errorf("low band: share want 0.40, got %v", got)
	}
	if low.Severity != models.SeverityLow {
		t.Errorf("share 0.40 → want SeverityLow, got %v", low.Severity)
	}

	med := soleFinding(t, depDetect(t, "package.json",
		`{"dependencies":{"a":"*","b":"*","c":"*","d":"1.0.0","e":"2.0.0"}}`)) // 3/5 = 0.60
	if got := shareOf(t, med); got != 0.60 {
		t.Errorf("medium band: share want 0.60, got %v", got)
	}
	if med.Severity != models.SeverityMedium {
		t.Errorf("share 0.60 → want SeverityMedium, got %v", med.Severity)
	}
}
