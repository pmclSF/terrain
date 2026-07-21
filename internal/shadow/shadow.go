// Package shadow is the sink for shadow-mode mechanism events.
//
// When a mechanism is in shadow state (per internal/mechanisms), each
// would-have-suppressed or would-have-added behavior change is logged to
// .terrain/shadow-report.jsonl. The user-visible findings are NOT
// affected.
//
// The shadow report records the behavior changes a shadow-state
// mechanism would have made if it were live, without affecting
// user-visible findings. The regression suites
// (internal/regressionsuite) and per-mechanism recall reports
// (internal/recallharness) consume this data.
//
// Sink behavior:
//   - One process-global sink, set via SetSink. Detectors call Emit which
//     dispatches to the active sink. Tests use NewMemorySink to capture
//     events.
//   - Writes are append-only JSONL with one event per line. The sink
//     flushes after every write so a crash mid-loop preserves
//     everything written so far (mirrors the policy in
//     scripts/_checkpoint.py).
//   - Disabled by default: if no sink is set, Emit is a cheap no-op.
//     The pipeline opts in by calling SetSink at startup when at least
//     one mechanism is in shadow state.
package shadow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Action describes what the mechanism would have done to a finding if it
// were live.
type Action string

const (
	// ActionSuppress means the mechanism would have removed an existing
	// finding (false-positive demotion).
	ActionSuppress Action = "would_suppress"

	// ActionAdd means the mechanism would have added a new finding
	// (recall recovery).
	ActionAdd Action = "would_add"

	// ActionDemoteSeverity means the mechanism would have lowered the
	// severity of an existing finding (e.g., catalog/example demotion).
	ActionDemoteSeverity Action = "would_demote_severity"
)

// Event is one shadow-mode observation. Each emit produces exactly one
// JSONL row in .terrain/shadow-report.jsonl.
type Event struct {
	// Timestamp is the time the event was emitted, in RFC3339.
	Timestamp string `json:"ts"`

	// Mechanism is the canonical name from internal/mechanisms (e.g.
	// "surface_literal_presence_gate").
	Mechanism string `json:"mechanism"`

	// RuleID is the consumer detector's rule_id.
	RuleID string `json:"rule_id"`

	// Action is what the mechanism would have done if live.
	Action Action `json:"action"`

	// File is the finding location (relative path). Optional.
	File string `json:"file,omitempty"`

	// Line is the finding line (1-indexed). Optional.
	Line int `json:"line,omitempty"`

	// Reasons is the structural justification — e.g., for ASCG, the
	// list of signals from internal/ascg.Classify.
	Reasons []string `json:"reasons,omitempty"`

	// Note is optional free-form context for debugging.
	Note string `json:"note,omitempty"`
}

// Sink is the interface every shadow-event consumer satisfies.
type Sink interface {
	Emit(Event) error
	Close() error
}

var (
	globalMu   sync.RWMutex
	globalSink Sink
)

// SetSink installs the process-global sink. Passing nil disables
// shadow logging. Returns the previous sink so callers can restore it
// (useful in tests).
func SetSink(s Sink) Sink {
	globalMu.Lock()
	defer globalMu.Unlock()
	prev := globalSink
	globalSink = s
	return prev
}

// Emit dispatches one shadow event to the active sink. No-op if no
// sink is set. Timestamp is filled in if the caller left it blank.
// Errors from the sink are swallowed because shadow logging must
// never fail an analyze run.
func Emit(e Event) {
	globalMu.RLock()
	s := globalSink
	globalMu.RUnlock()
	if s == nil {
		return
	}
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_ = s.Emit(e)
}

// FileSink writes events to .terrain/shadow-report.jsonl (or any other
// path) using a buffered writer. Sync happens at Close (and on demand
// via Flush); per-record fsync is replaced with periodic durability so
// a repo-wide scan emitting many events doesn't spend most of its time
// in fsync.
//
// Durability contract: Emit flushes the buffered writer after every
// event, so a crash mid-pipeline preserves everything written so far
// (mirrors the checkpoint policy used elsewhere in the codebase).
// Shadow events are sparse (one per gate firing, not per detection),
// so the per-event flush cost is bounded.
type FileSink struct {
	mu   sync.Mutex
	path string
	fh   *os.File
	bw   *bufio.Writer
	enc  *json.Encoder
}

// NewFileSink opens (or creates) the JSONL file at `path` in append
// mode. The parent directory is created if missing. Writes go through
// a 64 KiB buffered writer; callers MUST Close (or Flush) to durably
// persist the events.
func NewFileSink(path string) (*FileSink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create shadow dir: %w", err)
	}
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open shadow report: %w", err)
	}
	bw := bufio.NewWriterSize(fh, 64*1024)
	return &FileSink{
		path: path,
		fh:   fh,
		bw:   bw,
		enc:  json.NewEncoder(bw),
	}, nil
}

// Emit appends one JSON-encoded event line and flushes the buffered
// writer immediately. Callers do not need a separate Flush; Close
// still flushes + syncs as a final durability barrier.
func (f *FileSink) Emit(e Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fh == nil {
		return fmt.Errorf("FileSink: closed")
	}
	if err := f.enc.Encode(e); err != nil {
		return fmt.Errorf("encode shadow event: %w", err)
	}
	if f.bw != nil {
		if err := f.bw.Flush(); err != nil {
			return fmt.Errorf("flush shadow event: %w", err)
		}
	}
	return nil
}

// Flush forces buffered events to the underlying OS file (does not
// fsync). Callers that want a durability point mid-run call this.
func (f *FileSink) Flush() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bw == nil {
		return nil
	}
	return f.bw.Flush()
}

// Close flushes the buffer, syncs to disk, and closes the underlying
// file. Safe to call multiple times.
func (f *FileSink) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fh == nil {
		return nil
	}
	var flushErr error
	if f.bw != nil {
		flushErr = f.bw.Flush()
	}
	_ = f.fh.Sync()
	err := f.fh.Close()
	f.fh = nil
	f.bw = nil
	if flushErr != nil {
		return flushErr
	}
	return err
}

// Path returns the file path the sink writes to. Useful for diagnostics.
func (f *FileSink) Path() string { return f.path }

// MemorySink captures events in memory. Used by tests to assert on
// what the pipeline would have done.
type MemorySink struct {
	mu     sync.Mutex
	events []Event
}

// NewMemorySink returns a fresh in-memory sink.
func NewMemorySink() *MemorySink { return &MemorySink{} }

// Emit appends an event to the in-memory list.
func (m *MemorySink) Emit(e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// Close is a no-op for MemorySink.
func (m *MemorySink) Close() error { return nil }

// Events returns a defensive copy of every captured event.
func (m *MemorySink) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}

// Reset clears the captured events. Useful for table-driven tests that
// share one MemorySink across cases.
func (m *MemorySink) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

// WriteEvents is a convenience for callers that want to dump a snapshot
// of MemorySink events to an io.Writer in JSONL form.
func (m *MemorySink) WriteEvents(w io.Writer) error {
	for _, e := range m.Events() {
		if err := json.NewEncoder(w).Encode(e); err != nil {
			return err
		}
	}
	return nil
}
