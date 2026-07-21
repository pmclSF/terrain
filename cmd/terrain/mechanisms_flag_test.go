package main

import (
	"reflect"
	"testing"
)

func TestExtractMechanismOverrides_Equals(t *testing.T) {
	overrides, rest := extractMechanismOverrides([]string{
		"analyze",
		"--mechanisms.surface_literal_presence_gate=on",
		"--root", ".",
		"--mechanisms.barrel_resolver=shadow",
	})
	wantOverrides := []string{
		"surface_literal_presence_gate=on",
		"barrel_resolver=shadow",
	}
	wantRest := []string{"analyze", "--root", "."}
	if !reflect.DeepEqual(overrides, wantOverrides) {
		t.Errorf("overrides = %v, want %v", overrides, wantOverrides)
	}
	if !reflect.DeepEqual(rest, wantRest) {
		t.Errorf("rest = %v, want %v", rest, wantRest)
	}
}

func TestExtractMechanismOverrides_Space(t *testing.T) {
	overrides, rest := extractMechanismOverrides([]string{
		"analyze",
		"--mechanisms.surface_literal_presence_gate", "on",
		"--root", ".",
	})
	if !reflect.DeepEqual(overrides, []string{"surface_literal_presence_gate=on"}) {
		t.Errorf("overrides = %v", overrides)
	}
	if !reflect.DeepEqual(rest, []string{"analyze", "--root", "."}) {
		t.Errorf("rest = %v", rest)
	}
}

func TestExtractMechanismOverrides_None(t *testing.T) {
	args := []string{"analyze", "--root", ".", "--json"}
	overrides, rest := extractMechanismOverrides(args)
	if len(overrides) != 0 {
		t.Errorf("expected no overrides, got %v", overrides)
	}
	if !reflect.DeepEqual(rest, args) {
		t.Errorf("rest should equal input when no overrides; got %v", rest)
	}
}

func TestExtractMechanismOverrides_Malformed(t *testing.T) {
	// "--mechanisms.foo" with no value at end of args → captured as "foo="
	// so the registry layer raises the parse error.
	overrides, _ := extractMechanismOverrides([]string{"--mechanisms.foo"})
	if len(overrides) != 1 || overrides[0] != "foo=" {
		t.Errorf("malformed should pass through to runtime; got %v", overrides)
	}
}

func TestMechanismOverrides_DefensiveCopy(t *testing.T) {
	extractedMechanismOverrides = []string{"a=on", "b=shadow"}
	defer func() { extractedMechanismOverrides = nil }()
	got := mechanismOverrides()
	if len(got) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(got))
	}
	got[0] = "TAMPERED"
	if extractedMechanismOverrides[0] == "TAMPERED" {
		t.Errorf("mechanismOverrides should return a defensive copy")
	}
}

func TestMechanismOverrides_EmptyReturnsNil(t *testing.T) {
	extractedMechanismOverrides = nil
	if got := mechanismOverrides(); got != nil {
		t.Errorf("empty extracted should return nil, got %v", got)
	}
}
