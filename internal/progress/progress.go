// Package progress provides the unified spinner / stage-progress UI
// used across `terrain analyze`, `terrain migrate run`, `terrain ai
// run`, and `terrain report pr`. Track 10.5 of the 0.2.0 release
// plan calls for one progress vocabulary so adopters see the same
// shape regardless of which command is running.
//
// Design constraints:
//
//   - TTY-aware. When stderr is not a TTY (CI logs, pipes, file
//     redirects), every method is a no-op. Adopters running inside
//     CI never see spinner glyphs in their build logs.
//   - --quiet aware. Constructors take an explicit quiet flag.
//     A quiet Spinner is functionally equivalent to a non-TTY one.
//   - Stateless from the caller's perspective. The Stop method is
//     safe to call multiple times; Update is safe to call without
//     a matching Start.
//   - Zero dependencies on internal/uitokens at the package level
//     so Track 10.1's design tokens can themselves use progress
//     for long-running token-rendering operations without import
//     cycles. Symbol vocabulary is parallel but locally owned.
//
// Goes to stderr by default (not stdout) so JSON / report output
// piped to a file or another tool stays clean.
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// spinnerFrames is the canonical idle-progress glyph rotation.
// Using Braille pattern dots so the visual width is constant
// across frames; alternative animation (rotating slash, dots) is
// intentionally rejected because the slash is wide and dots can
// occupy variable widths depending on font rendering.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is a TTY-aware idle-progress indicator. Created with
// NewSpinner; only emits glyphs when stderr is a TTY and the
// caller didn't pass quiet=true.
type Spinner struct {
	out      io.Writer
	enabled  bool
	mu       sync.Mutex
	stop     chan struct{}
	done     chan struct{}
	label    string
	stopped  bool
}

// NewSpinner returns a Spinner that emits to stderr. quiet=true
// returns a no-op spinner regardless of TTY state. The TTY check
// is one-shot at construction time; subsequent stderr redirects
// don't change behavior.
func NewSpinner(label string, quiet bool) *Spinner {
	return newSpinner(os.Stderr, label, quiet, isTerminal(os.Stderr))
}

// newSpinner is the test-friendly constructor. Takes an explicit
// io.Writer + isTTY value so tests can inject a buffer and assert
// on output without needing a real terminal.
func newSpinner(out io.Writer, label string, quiet bool, isTTY bool) *Spinner {
	return &Spinner{
		out:     out,
		label:   label,
		enabled: !quiet && isTTY,
	}
}

// Start kicks off the animation goroutine. No-op if the spinner
// is disabled (not a TTY, --quiet, or already started).
func (s *Spinner) Start() {
	if s == nil || !s.enabled {
		return
	}
	s.mu.Lock()
	if s.stop != nil {
		// Already running — don't double-start. Calling Start a
		// second time is fine (defensive in callers that re-enter
		// long-running paths).
		s.mu.Unlock()
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.run()
}

func (s *Spinner) run() {
	defer close(s.done)
	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			s.clearLine()
			return
		case <-ticker.C:
			s.mu.Lock()
			label := s.label
			s.mu.Unlock()
			fmt.Fprintf(s.out, "\r%s %s", spinnerFrames[frame%len(spinnerFrames)], label)
			frame++
		}
	}
}

// Update changes the label shown next to the spinner. Safe to
// call from any goroutine, safe to call when the spinner is
// stopped or disabled (becomes a no-op).
func (s *Spinner) Update(label string) {
	if s == nil || !s.enabled {
		return
	}
	s.mu.Lock()
	s.label = label
	s.mu.Unlock()
}

// Stop ends the animation and clears the line. Idempotent — safe
// to call multiple times. Safe to call without a matching Start.
func (s *Spinner) Stop() {
	if s == nil || !s.enabled {
		return
	}
	s.mu.Lock()
	if s.stopped || s.stop == nil {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stop)
	done := s.done
	s.mu.Unlock()

	// Wait for the goroutine to clean up the line before returning
	// so the caller's next stderr write doesn't race.
	<-done
}

// clearLine wipes the current line so the next write doesn't have
// glyph residue. Two-step (carriage return + 80 spaces + carriage
// return) so it works on terminals that don't fully clear on \r.
func (s *Spinner) clearLine() {
	fmt.Fprintf(s.out, "\r%80s\r", "")
}

// Stage is a multi-step progress reporter for the canonical
// pipeline shape (Step 1/5 → Step 5/5). Used by analyze and ai run
// where the work is segmented into named stages.
type Stage struct {
	out     io.Writer
	enabled bool
	total   int
}

// NewStage returns a Stage progress reporter that writes to stderr.
func NewStage(total int, quiet bool) *Stage {
	return newStage(os.Stderr, total, quiet, isTerminal(os.Stderr))
}

func newStage(out io.Writer, total int, quiet bool, isTTY bool) *Stage {
	return &Stage{
		out:     out,
		total:   total,
		enabled: !quiet && isTTY,
	}
}

// Step prints "▸ Step n/total label" to stderr. No-op when
// disabled. Used by callers that want a discrete checkpoint
// rather than a spinning indicator.
func (s *Stage) Step(n int, label string) {
	if s == nil || !s.enabled {
		return
	}
	fmt.Fprintf(s.out, "▸ Step %d/%d  %s\n", n, s.total, label)
}

// Done prints a final "✓ Done." marker. No-op when disabled.
func (s *Stage) Done() {
	if s == nil || !s.enabled {
		return
	}
	fmt.Fprintln(s.out, "✓ Done.")
}

// isTerminal reports whether the given writer is a terminal. We
// only handle the *os.File case; arbitrary io.Writer is treated
// as non-terminal (correct default for buffers / pipes).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
