package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/pmclSF/terrain/internal/engine"
)

// runPipelineWithSignals wraps engine.RunPipelineContext with a
// SIGINT-aware context. Pre-0.2.x only `terrain analyze` honoured
// Ctrl-C; the other analysis commands (ai *, compare, explain, impact,
// insights *, report *) inherited engine.RunPipeline's
// context.Background and exited abruptly on Ctrl-C with no cleanup —
// leaving the user staring at a half-printed report and any in-flight
// detector still holding open file handles.
//
// Wrapping every callsite with this helper gives uniform interrupt
// semantics across the CLI surface. The cost is one extra goroutine
// per command invocation (signal.NotifyContext). The benefit is that
// `Ctrl-C` consistently means "unwind and exit", instead of "kill",
// which matters more on long monorepo scans where the user may want
// to abort mid-walk.
func runPipelineWithSignals(root string, opt engine.PipelineOptions) (*engine.PipelineResult, error) {
	return runPipelineWithSignalsAndTimeout(root, opt, 0)
}

// runPipelineWithSignalsAndTimeout extends runPipelineWithSignals with
// an optional timeout. When timeout > 0, the analysis context is
// cancelled after the duration elapses and the pipeline returns
// context.DeadlineExceeded. CI users running on large monorepos
// reach for this when an unbounded analysis would block their
// pipeline indefinitely.
func runPipelineWithSignalsAndTimeout(root string, opt engine.PipelineOptions, timeout time.Duration) (*engine.PipelineResult, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return engine.RunPipelineContext(ctx, root, opt)
}
