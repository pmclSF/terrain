package deps

import (
	"os"
	"path/filepath"
	"testing"

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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
