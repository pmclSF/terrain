package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/slash"
)

func TestRealDispatcher_Expand(t *testing.T) {
	dir := t.TempDir()
	terrainDir := filepath.Join(dir, ".terrain")
	if err := os.MkdirAll(terrainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	js := `{"version":1,"findings":[
		{"version":1,"rule_id":"terrain/coverage/no-tests","severity":"warning","primary_loc":{"path":"src/a.go","line":4},"short_message":"untested unit","docs_url":""},
		{"version":1,"rule_id":"terrain/ai/missing-eval","severity":"high","primary_loc":{"path":"prompts/p.md"},"short_message":"no eval","docs_url":""}
	]}`
	if err := os.WriteFile(filepath.Join(terrainDir, "findings.json"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}

	d := newRealDispatcher(dir)
	out, err := d.Handle(slash.WebhookEvent{}, &slash.Command{Verb: slash.VerbExpand})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, want := range []string{
		"All 2 findings",
		"terrain/coverage/no-tests", "src/a.go:4", "untested unit",
		"terrain/ai/missing-eval", "prompts/p.md", "no eval",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expand output missing %q:\n%s", want, out)
		}
	}

	// Missing artifact → polite guidance, never a dispatcher error.
	d2 := newRealDispatcher(t.TempDir())
	out2, err := d2.Handle(slash.WebhookEvent{}, &slash.Command{Verb: slash.VerbExpand})
	if err != nil {
		t.Fatalf("expand (missing artifact): %v", err)
	}
	if !strings.Contains(out2, "terrain analyze") {
		t.Errorf("missing-artifact reply unexpected: %s", out2)
	}
}
