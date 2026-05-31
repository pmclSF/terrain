package runtimeconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

func writeFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRecognizeFile_YAMLWithLoader(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, "config/model.yaml", `model: gpt-4o
temperature: 0.1
seed: 42
`)
	writeFile(t, root, "src/app.py", `import yaml
with open("config/model.yaml") as f:
    cfg = yaml.safe_load(f)`)

	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsRuntimeConfig() {
		t.Errorf("expected IsRuntimeConfig=true; got keys=%v loader=%v",
			r.ConfigKeysHit, r.HasLoaderInRepo)
	}
}

func TestRecognizeFile_YAMLNoLoader_NotRuntime(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, "config/model.yaml", `model: gpt-4o`)

	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsRuntimeConfig() {
		t.Errorf("config without consumer should NOT be classified as runtime")
	}
	if len(r.ConfigKeysHit) != 1 {
		t.Errorf("expected 1 hit (model), got %d", len(r.ConfigKeysHit))
	}
}

func TestRecognizeFile_NestedYAML(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, "app.yaml", `
service:
  ai:
    model: claude-3
    temperature: 0.0
`)
	writeFile(t, root, "src/app.py", `import yaml; yaml.safe_load("app.yaml")`)

	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsRuntimeConfig() {
		t.Errorf("nested-keyed YAML should still be classified as runtime; got %+v", r)
	}
	if len(r.ConfigKeysHit) < 2 {
		t.Errorf("expected ≥2 keys hit, got %d", len(r.ConfigKeysHit))
	}
}

func TestRecognizeFile_PropertiesFile(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, "application.properties", `
spring.ai.openai.chat.options.model=gpt-4o
spring.ai.openai.chat.options.temperature=0.2
spring.ai.openai.chat.options.seed=42
`)
	writeFile(t, root, "src/Main.java", `// loaded by Spring`)
	// Spring loads .properties implicitly. The recognizer requires a
	// loader pattern in code — without it, IsRuntimeConfig is false.
	// This test verifies the key extraction works regardless.
	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.ConfigKeysHit) < 3 {
		t.Errorf("expected ≥3 properties keys hit, got %d (%v)", len(r.ConfigKeysHit), r.ConfigKeysHit)
	}
}

func TestRecognizeFile_DotEnvWithLoader(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, ".env", `
MODEL=gpt-4o
TEMPERATURE=0.1
`)
	writeFile(t, root, "src/app.py", `from dotenv import load_dotenv
load_dotenv()`)

	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	// .env loader pattern (dotenv.load_dotenv) is in the recognizer's
	// loader regex but the file ext .env is properties-shaped.
	if len(r.ConfigKeysHit) < 2 {
		t.Errorf("expected env-var keys recognized, got %v", r.ConfigKeysHit)
	}
}

func TestRecognizeFile_NotAConfigFile(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, "data.yaml", `users:
  - name: alice
  - name: bob`)
	writeFile(t, root, "src/app.py", `import yaml; yaml.safe_load("data.yaml")`)

	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsRuntimeConfig() {
		t.Errorf("non-config YAML (no model keys) should NOT be classified as runtime")
	}
}

func TestRecognizeFile_UnknownExtension(t *testing.T) {
	root := t.TempDir()
	cfg := writeFile(t, root, "x.bin", `model: gpt`)
	r, err := RecognizeFile(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if r.Format != "unknown" {
		t.Errorf("unknown ext should set Format=unknown, got %q", r.Format)
	}
}

// ── GateDemotion ───────────────────────────────────────────────────

func loadReg(t *testing.T, state mechanisms.State) *mechanisms.Registry {
	t.Helper()
	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Override(MechanismName, state); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestGateDemotion_OffNeverDemotes(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOff)
	r := &Report{ConfigKeysHit: []string{"model"}, HasLoaderInRepo: true}
	if GateDemotion(reg, r, "rid", "f") {
		t.Errorf("state=off should never demote")
	}
}

func TestGateDemotion_OnDemotesRuntimeConfig(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	r := &Report{ConfigKeysHit: []string{"model"}, HasLoaderInRepo: true}
	if !GateDemotion(reg, r, "rid", "f") {
		t.Errorf("state=on + runtime config should demote")
	}
}

func TestGateDemotion_OnDoesNotDemoteNonRuntime(t *testing.T) {
	reg := loadReg(t, mechanisms.StateOn)
	r := &Report{ConfigKeysHit: []string{"model"}, HasLoaderInRepo: false}
	if GateDemotion(reg, r, "rid", "f") {
		t.Errorf("non-runtime config should not be demoted")
	}
}

func TestGateDemotion_ShadowEmitsEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := loadReg(t, mechanisms.StateShadow)
	r := &Report{ConfigKeysHit: []string{"model"}, HasLoaderInRepo: true}
	if GateDemotion(reg, r, "rid", "f") {
		t.Errorf("shadow should not demote user-visible findings")
	}
	if len(sink.Events()) != 1 {
		t.Errorf("expected 1 shadow event, got %d", len(sink.Events()))
	}
	if len(sink.Events()) == 1 && sink.Events()[0].Action != shadow.ActionDemoteSeverity {
		t.Errorf("event action = %v, want would_demote_severity", sink.Events()[0].Action)
	}
}
