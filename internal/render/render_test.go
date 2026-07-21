package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestVerdictLine(t *testing.T) {
	cases := map[int]string{
		0: "**Clear — nothing blocks this merge.**",
		1: "**1 finding blocks this merge.**", // singular
		2: "**2 findings block this merge.**", // first-plural boundary
		3: "**3 findings block this merge.**", // plural
	}
	for n, want := range cases {
		if got := VerdictLine(n); got != want {
			t.Errorf("VerdictLine(%d) = %q, want %q", n, got, want)
		}
	}
	if VerdictLine(-1) != VerdictLine(0) {
		t.Error("negative blocking count should read as clear")
	}
}

// Note: the severity legend's exact content is locked by the
// CommentHeader goldens (header-blocking.md / header-watch-only.md),
// which embed it verbatim — so a dropped or reworded label fails there.
// A separate "legend contains BLOCK/GATE/WATCH/NOTE" test would be
// redundant (asserting a constant contains literals), so it's omitted.

func TestProvenanceFooter(t *testing.T) {
	if got := ProvenanceFooter("0.4.0"); !strings.Contains(got, "Terrain 0.4.0") || !strings.Contains(got, "no API key") {
		t.Errorf("footer with version unexpected: %s", got)
	}
	if got := ProvenanceFooter(""); strings.Contains(got, "Terrain ") {
		t.Errorf("empty version should drop the version suffix: %s", got)
	}
}

func TestCommentHeader_Golden(t *testing.T) {
	cases := []struct {
		name            string
		blocking, total int
	}{
		{"clear", 0, 0},
		{"blocking", 1, 3},
		{"watch-only", 0, 2},
	}
	for _, c := range cases {
		assertGolden(t, "header-"+c.name+".md", CommentHeader(c.blocking, c.total))
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (regenerate with: go test ./internal/render -update): %v", name, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch %s\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}
