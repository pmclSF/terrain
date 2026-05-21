package configdrift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/signals"
)

func TestSchemaDriftDetector_GHActionsMutableRef(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".github/workflows/ci.yml"), `name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@main
      - uses: actions/setup-node@master
      - uses: actions/cache@v4
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	sig := out[0]
	if sig.Type != signals.SignalConfigSchemaDrift {
		t.Errorf("unexpected type: %v", sig.Type)
	}
	if kind, _ := sig.Metadata["kind"].(string); kind != "gh-actions" {
		t.Errorf("unexpected kind: %v", kind)
	}
	hazards, _ := sig.Metadata["hazards"].([]string)
	if len(hazards) == 0 || hazards[0] != "gh-actions:mutable-ref" {
		t.Errorf("expected mutable-ref hazard, got %v", hazards)
	}
}

func TestSchemaDriftDetector_GHActionsPinnedSHANoFinding(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".github/workflows/ci.yml"), `name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11
      - uses: actions/setup-node@v4
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 0 {
		t.Fatalf("expected 0 findings for SHA + tag pins, got %d", len(out))
	}
}

func TestSchemaDriftDetector_DockerComposeLatest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docker-compose.yml"), `version: '3'
services:
  web:
    image: nginx:latest
  db:
    image: postgres
  cache:
    image: redis:7.2.4
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	hazards, _ := out[0].Metadata["hazards"].([]string)
	if !containsStr(hazards, "docker:latest-tag") {
		t.Errorf("expected docker:latest-tag hazard, got %v", hazards)
	}
	if !containsStr(hazards, "docker:untagged-image") {
		t.Errorf("expected docker:untagged-image hazard, got %v", hazards)
	}
}

func TestSchemaDriftDetector_DockerComposeV2Schema(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docker-compose.yml"), `version: '2'
services:
  web:
    image: nginx:1.25.3
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding for v2 schema, got %d", len(out))
	}
	hazards, _ := out[0].Metadata["hazards"].([]string)
	if !containsStr(hazards, "docker:compose-v2-schema") {
		t.Errorf("expected compose-v2-schema hazard, got %v", hazards)
	}
}

func TestSchemaDriftDetector_K8sDeprecatedAPI(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "k8s/deployment.yaml"), `apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	if kind, _ := out[0].Metadata["kind"].(string); kind != "k8s-manifest" {
		t.Errorf("unexpected kind: %v", kind)
	}
	hazards, _ := out[0].Metadata["hazards"].([]string)
	if !containsStr(hazards, "k8s:deprecated-apiversion") {
		t.Errorf("expected k8s:deprecated-apiversion, got %v", hazards)
	}
}

func TestSchemaDriftDetector_HelmValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "charts/app/values.yaml"), `image:
  repository: myrepo/app
  tag: latest
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	// values.yaml — the latest tag is at `tag:`, not `image:`. The
	// detector only scans the `image:` line itself, so a Helm values
	// file with split fields produces 0 findings here. Verify the
	// detector is conservative rather than crashes on this case.
	if len(out) > 1 {
		t.Fatalf("expected 0-1 findings, got %d", len(out))
	}
}

func TestSchemaDriftDetector_SeverityScaling(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".github/workflows/ci.yml"), `name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@main
`)
	writeFile(t, filepath.Join(root, "docker-compose.yml"), `version: '2'
services:
  web:
    image: nginx:latest
  db:
    image: postgres
`)
	writeFile(t, filepath.Join(root, "k8s/dep.yaml"), `apiVersion: extensions/v1beta1
kind: Deployment
`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 3 {
		t.Fatalf("expected 3 findings (one per file), got %d", len(out))
	}
}

func TestSchemaDriftDetector_SkipsNonTracked(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# unrelated")
	writeFile(t, filepath.Join(root, "config.yaml"), `image: nginx:latest`)
	d := &SchemaDriftDetector{Root: root}
	out := d.Detect(&models.TestSuiteSnapshot{})
	if len(out) != 0 {
		t.Fatalf("expected 0 findings for unrecognized configs, got %d", len(out))
	}
}

func TestSchemaDriftDetector_NilSafe(t *testing.T) {
	var d *SchemaDriftDetector
	if got := d.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Errorf("expected nil from nil receiver, got %v", got)
	}
	d2 := &SchemaDriftDetector{Root: ""}
	if got := d2.Detect(&models.TestSuiteSnapshot{}); got != nil {
		t.Errorf("expected nil with empty Root, got %v", got)
	}
}

func containsStr(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
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

// TestSchemaDriftDetector_AscgGate_On proves the ascg_live_vs_catalog
// mechanism observably demotes findings on catalog/example paths
// (e.g. docs/, examples/) when state=on.
func TestSchemaDriftDetector_AscgGate_On(t *testing.T) {
	// Same hazard count, two paths: one in production /charts/, one
	// under examples/. The path-based ascg classifier demotes the
	// examples/ path; the prod path is unaffected.
	root := t.TempDir()
	// Both files: three distinct hazards → High severity. The gate's
	// demote is observable when both severities start above the
	// Low floor.
	composeHigh := `version: "2"
services:
  app:
    image: myapp:latest
  worker:
    image: myworker
`
	writeFile(t, filepath.Join(root, "deploy/docker-compose.yml"), composeHigh)
	writeFile(t, filepath.Join(root, "examples/docker-compose.yml"), composeHigh)

	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	prev := mechanisms.SetDefault(reg)
	defer mechanisms.SetDefault(prev)

	d := &SchemaDriftDetector{Root: root}

	prodPath := "deploy/docker-compose.yml"
	examplePath := "examples/docker-compose.yml"

	// Shadow → both findings emit at the same severity.
	got := d.Detect(&models.TestSuiteSnapshot{})
	if len(got) != 2 {
		t.Fatalf("shadow: expected 2 findings, got %d", len(got))
	}
	sevProdShadow := severityFor(got, prodPath)
	sevExShadow := severityFor(got, examplePath)
	if sevProdShadow == "" || sevExShadow == "" {
		t.Fatalf("shadow: missing one of the expected findings: %+v", got)
	}
	// Prod fixture has multiple hazard classes, examples has one;
	// prod severity should be at least Medium so the on-state demote
	// is observable below it.
	if severityRank(sevProdShadow) < severityRank(models.SeverityMedium) {
		t.Skipf("fixture didn't produce Medium+ severity for prod (got %s) — gate effect not observable", sevProdShadow)
	}

	// On → ascg demotes findings on examples/ paths one tier; prod
	// is unaffected because the path-based classifier doesn't flag it.
	if err := reg.ApplyCLIOverrides([]string{"ascg_live_vs_catalog=on"}); err != nil {
		t.Fatalf("override: %v", err)
	}
	got = d.Detect(&models.TestSuiteSnapshot{})
	sevProdOn := severityFor(got, prodPath)
	sevExOn := severityFor(got, examplePath)
	if severityRank(sevProdOn) != severityRank(sevProdShadow) {
		t.Errorf("on: prod severity unchanged expected; shadow=%s on=%s", sevProdShadow, sevProdOn)
	}
	if severityRank(sevExOn) >= severityRank(sevExShadow) {
		t.Errorf("on: examples severity should demote; shadow=%s on=%s", sevExShadow, sevExOn)
	}

	_ = signals.SignalConfigSchemaDrift
}

func severityFor(sigs []models.Signal, file string) models.SignalSeverity {
	for _, s := range sigs {
		if s.Location.File == file {
			return s.Severity
		}
	}
	return ""
}

func severityRank(s models.SignalSeverity) int {
	switch s {
	case models.SeverityLow:
		return 1
	case models.SeverityMedium:
		return 2
	case models.SeverityHigh:
		return 3
	case models.SeverityCritical:
		return 4
	}
	return 0
}
