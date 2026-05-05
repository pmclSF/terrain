package progress

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSpinner_DisabledOnNonTTY verifies the spinner emits nothing
// when isTTY is false, regardless of quiet flag. CI logs / piped
// stderr are the dominant case; spinner glyphs in those would be
// noise.
func TestSpinner_DisabledOnNonTTY(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := newSpinner(&buf, "scanning", false /*quiet*/, false /*isTTY*/)
	s.Start()
	time.Sleep(120 * time.Millisecond) // wait through one tick interval
	s.Update("still scanning")
	s.Stop()

	if buf.Len() != 0 {
		t.Errorf("non-TTY spinner emitted %d bytes; want 0:\n%q", buf.Len(), buf.String())
	}
}

// TestSpinner_DisabledByQuiet verifies --quiet suppresses output
// even on a TTY.
func TestSpinner_DisabledByQuiet(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := newSpinner(&buf, "scanning", true /*quiet*/, true /*isTTY*/)
	s.Start()
	time.Sleep(120 * time.Millisecond)
	s.Stop()

	if buf.Len() != 0 {
		t.Errorf("quiet spinner emitted %d bytes; want 0", buf.Len())
	}
}

// TestSpinner_EnabledOnTTYNotQuiet verifies the happy path: when
// stderr is a TTY and quiet is false, the spinner emits glyphs.
func TestSpinner_EnabledOnTTYNotQuiet(t *testing.T) {
	t.Parallel()
	// Wrap a buffer with a fake-TTY shim so the constructor's TTY
	// check returns true. Since newSpinner takes isTTY explicitly,
	// we just pass true.
	buf := &threadSafeBuffer{}
	s := newSpinner(buf, "scanning", false, true)
	s.Start()
	time.Sleep(200 * time.Millisecond) // let at least 2 frames render
	s.Stop()

	out := buf.String()
	// Expect at least one spinner glyph in the output.
	hasGlyph := false
	for _, frame := range spinnerFrames {
		if strings.Contains(out, frame) {
			hasGlyph = true
			break
		}
	}
	if !hasGlyph {
		t.Errorf("spinner output should contain a frame glyph; got %q", out)
	}
	// Expect the label.
	if !strings.Contains(out, "scanning") {
		t.Errorf("spinner output should contain label %q; got %q", "scanning", out)
	}
}

// TestSpinner_StopIdempotent verifies multiple Stop calls don't
// panic or double-close the channel.
func TestSpinner_StopIdempotent(t *testing.T) {
	t.Parallel()
	buf := &threadSafeBuffer{}
	s := newSpinner(buf, "x", false, true)
	s.Start()
	s.Stop()
	s.Stop() // second call should be a no-op
	s.Stop() // third call too
}

// TestSpinner_StopWithoutStart verifies calling Stop on a never-
// started spinner doesn't panic.
func TestSpinner_StopWithoutStart(t *testing.T) {
	t.Parallel()
	buf := &threadSafeBuffer{}
	s := newSpinner(buf, "x", false, true)
	s.Stop() // should be a no-op
}

// TestSpinner_NilSafe verifies all methods are safe on a nil
// spinner. Saves call sites from `if sp != nil { sp.Update(...) }`
// boilerplate.
func TestSpinner_NilSafe(t *testing.T) {
	t.Parallel()
	var s *Spinner
	s.Start()
	s.Update("x")
	s.Stop()
}

// TestStage_DisabledOnNonTTY verifies stage progress is silent on
// non-TTY (CI logs).
func TestStage_DisabledOnNonTTY(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := newStage(&buf, 5, false, false)
	s.Step(1, "scanning")
	s.Step(2, "analyzing")
	s.Done()

	if buf.Len() != 0 {
		t.Errorf("non-TTY stage emitted %d bytes; want 0", buf.Len())
	}
}

// TestStage_EnabledFormat verifies the canonical stage-output
// format: "▸ Step n/total label" + final "✓ Done.".
func TestStage_EnabledFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := newStage(&buf, 3, false, true)
	s.Step(1, "scanning")
	s.Step(2, "analyzing")
	s.Step(3, "writing")
	s.Done()

	out := buf.String()
	wantLines := []string{
		"▸ Step 1/3  scanning",
		"▸ Step 2/3  analyzing",
		"▸ Step 3/3  writing",
		"✓ Done.",
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("stage output missing %q; got:\n%s", want, out)
		}
	}
}

// TestStage_NilSafe verifies methods are safe on a nil stage.
func TestStage_NilSafe(t *testing.T) {
	t.Parallel()
	var s *Stage
	s.Step(1, "x")
	s.Done()
}

// threadSafeBuffer wraps bytes.Buffer with a mutex so the spinner
// goroutine and the test main goroutine can both access it
// concurrently without -race screams.
type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
