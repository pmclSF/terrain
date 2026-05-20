package surfacelit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/mechanisms"
	"github.com/pmclSF/terrain/internal/shadow"
)

// helper: load registry + flip the gate mechanism into the given state.
func gateReg(t *testing.T, state mechanisms.State) *mechanisms.Registry {
	t.Helper()
	reg, err := mechanisms.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := reg.Override(MechanismName, state); err != nil {
		t.Fatalf("Override: %v", err)
	}
	return reg
}

func writeFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGate_StateOff_AlwaysKeeps(t *testing.T) {
	reg := gateReg(t, mechanisms.StateOff)
	file := writeFile(t, `nothing matches here`)

	dec := Gate(reg, "gpt-4o", file, "aiModel")
	if !dec.Keep {
		t.Errorf("state=off should always keep, got Keep=false")
	}
}

func TestGate_StateShadow_NamePresent_KeepsNoEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := gateReg(t, mechanisms.StateShadow)
	file := writeFile(t, `model = "gpt-4o"`)

	dec := Gate(reg, "gpt-4o", file, "aiModel")
	if !dec.Keep {
		t.Errorf("state=shadow + present should keep")
	}
	if len(sink.Events()) != 0 {
		t.Errorf("present case should not emit shadow event, got %d", len(sink.Events()))
	}
}

func TestGate_StateShadow_NameAbsent_KeepsEmitsEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := gateReg(t, mechanisms.StateShadow)
	file := writeFile(t, `model = "claude-3-opus"`)

	dec := Gate(reg, "gpt-4o", file, "aiModel")
	if !dec.Keep {
		t.Errorf("state=shadow should keep even when absent")
	}
	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 shadow event, got %d", len(events))
	}
	e := events[0]
	if e.Mechanism != MechanismName {
		t.Errorf("event.Mechanism = %q", e.Mechanism)
	}
	if e.RuleID != "aiModel" {
		t.Errorf("event.RuleID = %q", e.RuleID)
	}
	if e.Action != shadow.ActionSuppress {
		t.Errorf("event.Action = %v", e.Action)
	}
}

func TestGate_StateOn_NameAbsent_DropsNoEvent(t *testing.T) {
	sink := shadow.NewMemorySink()
	prev := shadow.SetSink(sink)
	t.Cleanup(func() { shadow.SetSink(prev) })

	reg := gateReg(t, mechanisms.StateOn)
	file := writeFile(t, `model = "claude-3-opus"`)

	dec := Gate(reg, "gpt-4o", file, "aiModel")
	if dec.Keep {
		t.Errorf("state=on + absent should drop (Keep=false)")
	}
	if len(sink.Events()) != 0 {
		t.Errorf("state=on does not emit shadow events, got %d", len(sink.Events()))
	}
}

func TestGate_StateOn_NamePresent_Keeps(t *testing.T) {
	reg := gateReg(t, mechanisms.StateOn)
	file := writeFile(t, `model = "gpt-4o"`)

	dec := Gate(reg, "gpt-4o", file, "aiModel")
	if !dec.Keep {
		t.Errorf("state=on + present should keep")
	}
}

func TestGate_MissingFile_FailsOpen(t *testing.T) {
	reg := gateReg(t, mechanisms.StateOn)

	dec := Gate(reg, "gpt-4o", "/no/such/file", "aiModel")
	if !dec.Keep {
		t.Errorf("missing file should fail open (Keep=true), got Keep=false")
	}
}

func TestGate_EmptyName_FailsOpen(t *testing.T) {
	reg := gateReg(t, mechanisms.StateOn)
	file := writeFile(t, "anything")
	dec := Gate(reg, "", file, "aiModel")
	if !dec.Keep {
		t.Errorf("empty name should fail open")
	}
}
