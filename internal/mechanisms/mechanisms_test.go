package mechanisms

import (
	"strings"
	"sync"
	"testing"
)

func TestLoad_EmbeddedYAMLParses(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d", reg.SchemaVersion)
	}
	// Baseline must define at least the P2 mechanisms.
	names := reg.Names()
	mustInclude := []string{
		"surface_literal_presence_gate",
		"a7_barrel_resolver",
		"ascg_live_vs_catalog",
	}
	for _, want := range mustInclude {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("mechanisms.yaml missing %q (have %v)", want, names)
		}
	}
}

func TestLoad_NewMechanismsStartShadow(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Binding rule: new mechanisms ship as shadow. Mechanisms that
	// have graduated to live are explicitly listed in liveMechanisms;
	// every other mechanism must remain at shadow.
	liveMechanisms := map[string]bool{
		"deprecated_test_pattern_trigger_gate": true,
		"surface_literal_presence_gate":        true,
		"ascg_live_vs_catalog":                 true,
	}
	for _, m := range reg.All() {
		if liveMechanisms[m.Name] {
			if m.State != StateOn {
				t.Errorf("%s is on the live list but loaded at %s", m.Name, m.State)
			}
			continue
		}
		if m.State != StateShadow {
			t.Errorf("%s starts at %s; binding rule requires shadow for new mechanisms", m.Name, m.State)
		}
	}
}

func TestParseState(t *testing.T) {
	cases := map[string]State{
		"":       StateOff,
		"off":    StateOff,
		"OFF":    StateOff,
		"shadow": StateShadow,
		"on":     StateOn,
		"  on  ": StateOn,
	}
	for in, want := range cases {
		got, err := ParseState(in)
		if err != nil {
			t.Errorf("ParseState(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseState(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseState("garbage"); err == nil {
		t.Errorf("expected error for garbage state")
	}
}

func TestLoadFromBytes_RejectsBadSchema(t *testing.T) {
	bad := []byte(`schema_version: 99
mechanisms: []`)
	_, err := LoadFromBytes(bad)
	if err == nil || !strings.Contains(err.Error(), "schema_version 99") {
		t.Errorf("expected schema-version error, got %v", err)
	}
}

func TestLoadFromBytes_RejectsDuplicate(t *testing.T) {
	dup := []byte(`schema_version: 1
mechanisms:
  - name: foo
    state: shadow
    description: x
  - name: foo
    state: shadow
    description: y`)
	_, err := LoadFromBytes(dup)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestLoadFromBytes_RejectsUnknownState(t *testing.T) {
	bad := []byte(`schema_version: 1
mechanisms:
  - name: foo
    state: maybe
    description: x`)
	_, err := LoadFromBytes(bad)
	if err == nil || !strings.Contains(err.Error(), "unknown state") {
		t.Errorf("expected unknown-state error, got %v", err)
	}
}

func TestState_UnknownNameReturnsOff(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := reg.State("does_not_exist"); got != StateOff {
		t.Errorf("unknown mechanism state = %v, want off", got)
	}
}

func TestOverride(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	const name = "surface_literal_presence_gate"
	if err := reg.Override(name, StateOn); err != nil {
		t.Fatalf("Override: %v", err)
	}
	if got := reg.State(name); got != StateOn {
		t.Errorf("after Override(%s, on), state = %v", name, got)
	}
	if err := reg.Override("does_not_exist", StateOn); err == nil {
		t.Errorf("expected error overriding unknown mechanism")
	}
}

func TestApplyCLIOverrides(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	overrides := []string{
		"surface_literal_presence_gate=on",
		"a7_barrel_resolver=off",
	}
	if err := reg.ApplyCLIOverrides(overrides); err != nil {
		t.Fatalf("ApplyCLIOverrides: %v", err)
	}
	if reg.State("surface_literal_presence_gate") != StateOn {
		t.Errorf("surface_literal_presence_gate not flipped to on")
	}
	if reg.State("a7_barrel_resolver") != StateOff {
		t.Errorf("a7_barrel_resolver not flipped to off")
	}
}

func TestApplyCLIOverrides_RejectsMalformed(t *testing.T) {
	reg, _ := Load()
	for _, bad := range []string{
		"no_equals_sign",
		"=missing_name",
		"name=garbage_state",
	} {
		if err := reg.ApplyCLIOverrides([]string{bad}); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestRegistry_NilSafe(t *testing.T) {
	var r *Registry
	if got := r.State("anything"); got != StateOff {
		t.Errorf("nil State should return off, got %v", got)
	}
	if got := r.Get("anything"); got != nil {
		t.Errorf("nil Get should return nil, got %v", got)
	}
	if err := r.Override("anything", StateOn); err == nil {
		t.Errorf("nil Override should error")
	}
	if names := r.Names(); names != nil {
		t.Errorf("nil Names should return nil, got %v", names)
	}
}

func TestRegistry_ConcurrentReadOverride(t *testing.T) {
	reg, _ := Load()
	const name = "surface_literal_presence_gate"
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = reg.State(name)
		}()
		go func() {
			defer wg.Done()
			_ = reg.Override(name, StateShadow)
		}()
	}
	wg.Wait()
}
