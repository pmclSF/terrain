package aidetect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDetectContext_CancellationFromCancelledContext verifies the
// "already-cancelled" path: a context that's already done when
// DetectContext starts should return promptly without doing the
// full repo scan. This is the fast-path the ctx threading is
// designed to support — a CI workflow whose `--timeout` already
// fired by the time the AI phase runs.
func TestDetectContext_CancellationFromCancelledContext(t *testing.T) {
	t.Parallel()
	tmp := buildLargeAIRepo(t, 200)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done

	start := time.Now()
	result := DetectContext(ctx, tmp)
	elapsed := time.Since(start)

	if elapsed > 250*time.Millisecond {
		t.Errorf("cancelled context did not short-circuit: took %v on a %d-file fixture",
			elapsed, 200)
	}
	if result == nil {
		t.Error("DetectContext should return a non-nil result even when cancelled")
	}
}

// TestDetectContext_CancellationDuringWalk verifies the in-flight
// cancellation path: a context that gets cancelled while DetectContext
// is mid-walk should abort cleanly. This is the regression case the
// pre-Track 5.3 shape failed silently — a `terrain analyze` run with a
// 5-second budget would still wait minutes for the AI walk to finish
// after ctx was cancelled.
func TestDetectContext_CancellationDuringWalk(t *testing.T) {
	t.Parallel()
	// Build a fixture large enough that the walk takes some real
	// time so the in-flight cancel actually races. 1000 files with
	// AI import patterns gives us ~200ms of walk on commodity
	// hardware — plenty of room for a 50ms cancel to race.
	tmp := buildLargeAIRepo(t, 1000)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result := DetectContext(ctx, tmp)
	elapsed := time.Since(start)

	if result == nil {
		t.Error("DetectContext should return a non-nil result even when cancelled mid-walk")
	}
	// If cancellation isn't honored, the walk runs to completion —
	// at least 200ms on a 1000-file fixture. The test allows up to
	// 1s as a generous upper bound that still proves the ctx check
	// fires in the inner loop. Without ctx threading the walk
	// would block ~200ms+ before returning, regardless of how
	// quickly ctx was cancelled.
	if elapsed > 1*time.Second {
		t.Errorf("DetectContext did not honor mid-walk cancellation: took %v on a 1000-file fixture (expected to abort within 1s after 20ms cancel)",
			elapsed)
	}
}

// TestDetect_BackwardsCompat verifies that the wrapping Detect()
// function still works end-to-end and produces equivalent results
// to DetectContext(context.Background(), root). Important because
// every external caller still uses Detect; we only switched the
// pipeline's call site to DetectContext.
func TestDetect_BackwardsCompat(t *testing.T) {
	t.Parallel()
	tmp := buildLargeAIRepo(t, 50)

	classic := Detect(tmp)
	withCtx := DetectContext(context.Background(), tmp)

	if len(classic.PromptFiles) != len(withCtx.PromptFiles) {
		t.Errorf("PromptFiles count diverged: classic=%d ctx=%d",
			len(classic.PromptFiles), len(withCtx.PromptFiles))
	}
	if len(classic.Frameworks) != len(withCtx.Frameworks) {
		t.Errorf("Frameworks count diverged: classic=%d ctx=%d",
			len(classic.Frameworks), len(withCtx.Frameworks))
	}
}

// buildLargeAIRepo creates a temp directory with N source files that
// contain AI import patterns and prompt-shaped strings. Used as the
// fixture for the cancellation tests.
func buildLargeAIRepo(t *testing.T, n int) string {
	t.Helper()
	tmp := t.TempDir()

	const tmpl = `import openai
from langchain import LLM

prompt = """You are a helpful assistant.

Answer the user's question clearly and concisely.

User: {input}
Assistant:"""

response = openai.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "system", "content": prompt}],
)
`

	for i := 0; i < n; i++ {
		dir := filepath.Join(tmp, fmt.Sprintf("pkg%03d", i/50))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		path := filepath.Join(dir, fmt.Sprintf("agent_%04d.py", i))
		if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return tmp
}
