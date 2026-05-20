package shadow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestEmit_NoSinkIsNoop(t *testing.T) {
	// Make sure no sink is set.
	prev := SetSink(nil)
	t.Cleanup(func() { SetSink(prev) })

	// Should not panic, should not error.
	Emit(Event{Mechanism: "test", RuleID: "r1", Action: ActionSuppress})
}

func TestEmit_DispatchesToMemorySink(t *testing.T) {
	sink := NewMemorySink()
	prev := SetSink(sink)
	t.Cleanup(func() { SetSink(prev) })

	Emit(Event{Mechanism: "m1", RuleID: "r1", Action: ActionSuppress, File: "a.py", Line: 10})
	Emit(Event{Mechanism: "m1", RuleID: "r2", Action: ActionAdd, File: "b.py"})

	events := sink.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Mechanism != "m1" || events[0].File != "a.py" || events[0].Line != 10 {
		t.Errorf("event[0] mismatch: %+v", events[0])
	}
	if events[1].Action != ActionAdd {
		t.Errorf("event[1].Action = %v", events[1].Action)
	}
	// Timestamps should be auto-filled.
	for i, e := range events {
		if e.Timestamp == "" {
			t.Errorf("event[%d] has empty timestamp", i)
		}
	}
}

func TestEmit_RespectsCallerTimestamp(t *testing.T) {
	sink := NewMemorySink()
	prev := SetSink(sink)
	t.Cleanup(func() { SetSink(prev) })

	Emit(Event{Timestamp: "2026-01-01T00:00:00Z", Mechanism: "m"})
	if got := sink.Events()[0].Timestamp; got != "2026-01-01T00:00:00Z" {
		t.Errorf("Timestamp = %q, want preserved", got)
	}
}

func TestFileSink_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".terrain", "shadow-report.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	events := []Event{
		{Mechanism: "m1", RuleID: "r1", Action: ActionSuppress, File: "a.py"},
		{Mechanism: "m1", RuleID: "r2", Action: ActionAdd, Reasons: []string{"foo", "bar"}},
	}
	for _, e := range events {
		if err := sink.Emit(e); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shadow report: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var first Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("parse line 0: %v", err)
	}
	if first.Mechanism != "m1" || first.File != "a.py" {
		t.Errorf("line 0 round-trip mismatch: %+v", first)
	}
}

func TestFileSink_AppendsAcrossReopens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shadow.jsonl")

	s1, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("first NewFileSink: %v", err)
	}
	_ = s1.Emit(Event{Mechanism: "m1"})
	_ = s1.Close()

	s2, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("second NewFileSink: %v", err)
	}
	_ = s2.Emit(Event{Mechanism: "m2"})
	_ = s2.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected append (2 lines), got %d: %s", len(lines), string(data))
	}
}

func TestFileSink_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "nested", "path", "shadow.jsonl")
	sink, err := NewFileSink(nested)
	if err != nil {
		t.Fatalf("NewFileSink with nested path: %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })
	if _, err := os.Stat(filepath.Dir(nested)); err != nil {
		t.Errorf("parent dir not created: %v", err)
	}
}

func TestFileSink_EmitAfterClose(t *testing.T) {
	dir := t.TempDir()
	sink, _ := NewFileSink(filepath.Join(dir, "x.jsonl"))
	_ = sink.Close()
	if err := sink.Emit(Event{Mechanism: "m"}); err == nil {
		t.Errorf("Emit after Close should error")
	}
}

func TestSetSink_ReturnsPrevious(t *testing.T) {
	sinkA := NewMemorySink()
	sinkB := NewMemorySink()
	prev0 := SetSink(sinkA)
	t.Cleanup(func() { SetSink(prev0) })

	prev := SetSink(sinkB)
	if prev != Sink(sinkA) {
		t.Errorf("SetSink should return previous sink")
	}
}

func TestMemorySink_Reset(t *testing.T) {
	s := NewMemorySink()
	_ = s.Emit(Event{Mechanism: "m"})
	s.Reset()
	if got := s.Events(); len(got) != 0 {
		t.Errorf("expected 0 events after Reset, got %d", len(got))
	}
}

func TestMemorySink_WriteEvents(t *testing.T) {
	s := NewMemorySink()
	_ = s.Emit(Event{Mechanism: "m1", RuleID: "r1"})
	_ = s.Emit(Event{Mechanism: "m2", RuleID: "r2"})

	var buf bytes.Buffer
	if err := s.WriteEvents(&buf); err != nil {
		t.Fatalf("WriteEvents: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 JSONL lines, got %d", len(lines))
	}
}

func TestEmit_ConcurrentSafe(t *testing.T) {
	sink := NewMemorySink()
	prev := SetSink(sink)
	t.Cleanup(func() { SetSink(prev) })

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Emit(Event{Mechanism: "m", RuleID: "r"})
		}(i)
	}
	wg.Wait()
	if got := len(sink.Events()); got != 100 {
		t.Errorf("expected 100 events, got %d", got)
	}
}
