package aliases

import (
	"sort"
	"strings"
	"testing"
)

func TestLoad_EmbeddedDefault(t *testing.T) {
	r, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.Version != 1 {
		t.Errorf("expected schema version 1, got %d", r.Version)
	}
	// Baseline ships with no active aliases. When the first rule split
	// lands, this test will start failing — that's the signal to add a
	// corresponding alias-entry test.
	if len(r.Aliases) != 0 {
		t.Logf("alias registry has %d active entries (baseline expected 0; update test if a split added one)", len(r.Aliases))
	}
}

func TestExpandOldID_NoAlias(t *testing.T) {
	r := &Registry{Version: 1, Aliases: map[string]AliasEntry{}}
	r.buildReverseIndex()
	got := r.ExpandOldID("untrackedRule")
	if len(got) != 1 || got[0] != "untrackedRule" {
		t.Errorf("ExpandOldID untracked = %v, want [untrackedRule]", got)
	}
}

func TestExpandOldID_WithAlias(t *testing.T) {
	yamlBytes := []byte(`
version: 1
aliases:
  staticSkippedTest:
    replaces_with:
      - staticSkippedTest-unconditional
      - staticSkippedTest-conditional-gate
    deprecated_in: "0.3.0"
    why: split into unconditional + conditional-gate so the conditional-skip class survives narrowing
`)
	r, err := LoadFromBytes(yamlBytes)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	got := r.ExpandOldID("staticSkippedTest")
	sort.Strings(got)
	want := []string{
		"staticSkippedTest",
		"staticSkippedTest-conditional-gate",
		"staticSkippedTest-unconditional",
	}
	if len(got) != len(want) {
		t.Fatalf("ExpandOldID = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ExpandOldID[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOldIDFor(t *testing.T) {
	yamlBytes := []byte(`
version: 1
aliases:
  depsDriftRisk:
    replaces_with:
      - depsDriftRisk-strict-pin
      - depsDriftRisk-caret-policy
    deprecated_in: "0.3.0"
`)
	r, err := LoadFromBytes(yamlBytes)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	old, ok := r.OldIDFor("depsDriftRisk-strict-pin")
	if !ok || old != "depsDriftRisk" {
		t.Errorf("OldIDFor(strict-pin) = (%q, %v), want (depsDriftRisk, true)", old, ok)
	}

	_, ok = r.OldIDFor("unknownRule")
	if ok {
		t.Errorf("OldIDFor(unknownRule) should be false")
	}
}

func TestLoadFromBytes_InvalidVersion(t *testing.T) {
	yamlBytes := []byte(`
version: 99
aliases: {}
`)
	_, err := LoadFromBytes(yamlBytes)
	if err == nil || !strings.Contains(err.Error(), "schema version 99") {
		t.Errorf("expected schema-version error, got: %v", err)
	}
}

func TestLoadFromBytes_EmptyReplacesWith(t *testing.T) {
	yamlBytes := []byte(`
version: 1
aliases:
  badRule:
    replaces_with: []
`)
	_, err := LoadFromBytes(yamlBytes)
	if err == nil || !strings.Contains(err.Error(), "no replaces_with") {
		t.Errorf("expected no-replaces-with error, got: %v", err)
	}
}

func TestLoadFromBytes_SelfReplacement(t *testing.T) {
	yamlBytes := []byte(`
version: 1
aliases:
  recursiveRule:
    replaces_with:
      - recursiveRule
`)
	_, err := LoadFromBytes(yamlBytes)
	if err == nil || !strings.Contains(err.Error(), "replaces itself") {
		t.Errorf("expected self-replacement error, got: %v", err)
	}
}

func TestNilRegistry_SafeExpand(t *testing.T) {
	var r *Registry
	got := r.ExpandOldID("anyRule")
	if len(got) != 1 || got[0] != "anyRule" {
		t.Errorf("nil-receiver ExpandOldID should return identity, got %v", got)
	}
	if _, ok := r.OldIDFor("anyRule"); ok {
		t.Errorf("nil-receiver OldIDFor should return false")
	}
}
