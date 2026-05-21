package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunMechanismsList_Text(t *testing.T) {
	var buf bytes.Buffer
	if err := runMechanismsList(&buf, false); err != nil {
		t.Fatalf("runMechanismsList: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected NAME header, got %q", out)
	}
	if !strings.Contains(out, "surface_literal_presence_gate") {
		t.Errorf("expected at least one mechanism listed, got %q", out)
	}
	if !strings.Contains(out, "--mechanisms.<name>=on|off|shadow") {
		t.Errorf("output should explain the override flag")
	}
}

func TestRunMechanismsList_JSON(t *testing.T) {
	var buf bytes.Buffer
	if err := runMechanismsList(&buf, true); err != nil {
		t.Fatalf("runMechanismsList: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("json.Unmarshal: %v\nbody: %s", err, buf.String())
	}
	if len(entries) < 5 {
		t.Errorf("expected ≥5 mechanisms in JSON list, got %d", len(entries))
	}
	for _, e := range entries {
		if _, ok := e["name"]; !ok {
			t.Errorf("entry missing name field: %v", e)
		}
		if _, ok := e["state"]; !ok {
			t.Errorf("entry missing state field: %v", e)
		}
	}
}

func TestRunMechanismsShow_Existing(t *testing.T) {
	var buf bytes.Buffer
	if err := runMechanismsShow(&buf, "surface_literal_presence_gate"); err != nil {
		t.Fatalf("runMechanismsShow: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "surface_literal_presence_gate") {
		t.Errorf("show output missing mechanism name")
	}
	if !strings.Contains(out, "Description:") {
		t.Errorf("show output missing Description header")
	}
	if !strings.Contains(out, "Consumer rule_ids:") {
		t.Errorf("show output missing consumers header")
	}
}

func TestRunMechanismsShow_Unknown(t *testing.T) {
	var buf bytes.Buffer
	err := runMechanismsShow(&buf, "not_a_real_mechanism")
	if err == nil {
		t.Errorf("expected error for unknown mechanism")
		return
	}
	if !strings.Contains(err.Error(), "unknown mechanism") {
		t.Errorf("error should say 'unknown mechanism', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Available mechanisms") {
		t.Errorf("error should list available mechanisms")
	}
}

func TestRunMechanismsCLI_DefaultsToList(t *testing.T) {
	// No args → defaults to list (writes to os.Stdout — just check no error)
	if err := runMechanismsCLI(nil); err != nil {
		t.Errorf("runMechanismsCLI([]) should default to list; got error %v", err)
	}
}

func TestRunMechanismsCLI_UnknownVerbErrors(t *testing.T) {
	err := runMechanismsCLI([]string{"banana"})
	if err == nil {
		t.Errorf("expected error for unknown verb")
	}
}

func TestRunMechanismsCLI_ShowRequiresName(t *testing.T) {
	err := runMechanismsCLI([]string{"show"})
	if err == nil || !strings.Contains(err.Error(), "show <name>") {
		t.Errorf("show without name should error with usage; got %v", err)
	}
}
