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

// TestSchemaDriftDetector_NotDemotedInExamples proves
// configSchemaDrift findings on `examples/` paths are NOT demoted by
// the live-vs-catalog classifier. Docker-compose / k8s / helm files in
// `examples/` directories are typically deployable artifacts —
// `:latest` tags in them are still forward-compat hazards regardless
// of path.
func TestSchemaDriftDetector_NotDemotedInExamples(t *testing.T) {
	root := t.TempDir()
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

	// Flip ascg to ON — should have NO effect on configSchemaDrift.
	if err := reg.ApplyCLIOverrides([]string{"ascg_live_vs_catalog=on"}); err != nil {
		t.Fatalf("override: %v", err)
	}

	d := &SchemaDriftDetector{Root: root}
	got := d.Detect(&models.TestSuiteSnapshot{})
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(got))
	}
	sevProd := severityFor(got, "deploy/docker-compose.yml")
	sevEx := severityFor(got, "examples/docker-compose.yml")
	if sevProd != sevEx {
		t.Errorf("expected matching severity prod=%s vs examples=%s — configSchemaDrift should not be demoted by ascg",
			sevProd, sevEx)
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

func cdDetect(t *testing.T, path, content string) []models.Signal {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, path), content)
	return (&SchemaDriftDetector{Root: root}).Detect(&models.TestSuiteSnapshot{})
}

func oneFinding(t *testing.T, sigs []models.Signal) models.Signal {
	t.Helper()
	if len(sigs) != 1 {
		t.Fatalf("want exactly 1 finding, got %d: %+v", len(sigs), sigs)
	}
	return sigs[0]
}

func hazardsOf(sig models.Signal) []string {
	h, _ := sig.Metadata["hazards"].([]string)
	return h
}

func hasHazard(hs []string, want string) bool {
	for _, h := range hs {
		if h == want {
			return true
		}
	}
	return false
}

// CD8: severity scales with the NUMBER OF DISTINCT hazards in a file —
// 1→Low, 2→Medium, ≥3→High. The original severity test never asserts severity.
func TestSchemaDriftDetector_SeverityByHazardCount(t *testing.T) {
	t.Parallel()
	one := oneFinding(t, cdDetect(t, "docker-compose.yml",
		"version: '3'\nservices:\n  web:\n    image: nginx:latest\n"))
	if one.Severity != models.SeverityLow {
		t.Errorf("1 hazard (latest) → Low, got %v", one.Severity)
	}
	two := oneFinding(t, cdDetect(t, "docker-compose.yml",
		"version: '3'\nservices:\n  web:\n    image: nginx:latest\n  db:\n    image: postgres\n"))
	if two.Severity != models.SeverityMedium {
		t.Errorf("2 hazards (latest+untagged) → Medium, got %v", two.Severity)
	}
	three := oneFinding(t, cdDetect(t, "docker-compose.yml",
		"version: '2'\nservices:\n  web:\n    image: nginx:latest\n  db:\n    image: postgres\n"))
	if three.Severity != models.SeverityHigh {
		t.Errorf("3 hazards (v2+latest+untagged) → High, got %v", three.Severity)
	}
}

// CD2: multiple :latest images collapse to ONE hazard tag (not per-occurrence).
func TestSchemaDriftDetector_HazardDedup(t *testing.T) {
	t.Parallel()
	sig := oneFinding(t, cdDetect(t, "docker-compose.yml",
		"version: '3'\nservices:\n  a:\n    image: nginx:latest\n  b:\n    image: redis:latest\n  c:\n    image: app:latest\n"))
	n := 0
	for _, h := range hazardsOf(sig) {
		if h == "docker:latest-tag" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("3 :latest images must dedup to a single hazard tag, got %d (%v)", n, hazardsOf(sig))
	}
	if sig.Severity != models.SeverityLow {
		t.Errorf("1 distinct hazard → Low, got %v", sig.Severity)
	}
}

// CD4: version: "2" (double quotes) flags compose-v2-schema like single quotes.
func TestSchemaDriftDetector_ComposeV2DoubleQuote(t *testing.T) {
	t.Parallel()
	sig := oneFinding(t, cdDetect(t, "docker-compose.yml",
		"version: \"2\"\nservices:\n  web:\n    image: nginx:1.0\n"))
	if !hasHazard(hazardsOf(sig), "docker:compose-v2-schema") {
		t.Errorf("version: \"2\" must flag compose-v2-schema; hazards=%v", hazardsOf(sig))
	}
}

// CD1: an action ref with no @ at all is a mutable ref.
func TestSchemaDriftDetector_GHActionUntaggedRef(t *testing.T) {
	t.Parallel()
	sig := oneFinding(t, cdDetect(t, ".github/workflows/ci.yml",
		"name: ci\non: [push]\njobs:\n  j:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout\n"))
	if !hasHazard(hazardsOf(sig), "gh-actions:mutable-ref") {
		t.Errorf("untagged action ref must flag mutable-ref; hazards=%v", hazardsOf(sig))
	}
}

// CD3: a registry:port image WITH a tag is not flagged untagged; only the
// genuinely untagged image is (the port colon must not be mistaken for a tag).
func TestSchemaDriftDetector_PortAwareUntagged(t *testing.T) {
	t.Parallel()
	sig := oneFinding(t, cdDetect(t, "docker-compose.yml",
		"version: '3'\nservices:\n  a:\n    image: localhost:5000/myapp:v1.0\n  b:\n    image: localhost:5000/other\n"))
	if !hasHazard(hazardsOf(sig), "docker:untagged-image") {
		t.Errorf("localhost:5000/other (no tag) must be flagged untagged; hazards=%v", hazardsOf(sig))
	}
	if sig.Severity != models.SeverityLow {
		t.Errorf("only one image is genuinely untagged → 1 hazard → Low, got %v", sig.Severity)
	}
}
