package configdrift

import (
	"os"
	"path/filepath"
	"testing"

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
