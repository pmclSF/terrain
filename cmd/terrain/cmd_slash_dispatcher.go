package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/engine"
	"github.com/pmclSF/terrain/internal/findinghistory"
	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/slash"
)

// realDispatcher implements slash.Dispatcher by routing each verb to
// the existing CLI runner that already implements the behavior. This
// is what production webhook deployments use; DefaultDispatcher
// (informational only) stays in internal/slash so the package can be
// tested without depending on cmd/terrain.
//
// Routing:
//
//	/dismiss              → runSuppress (writes a content-hash entry)
//	/terrain show <id>    → runReportShow (renders one finding)
//	/terrain explain <id> → runReportExplain (long-form rule docs)
//	/terrain why <id>     → runReportExplain --short
//	/terrain commands     → slash.RenderCommandList
//	/terrain refresh      → noop placeholder (would re-run analyze;
//	                         deferred until the snapshot-cache lands)
//	/terrain expand       → noop placeholder (depends on the comment-
//	                         ID that posted the collapsed block)
//	/terrain escalate     → noop placeholder (needs PR-state machinery)
//	/terrain scaffold     → noop placeholder (needs scaffold engine)
//	/terrain bench        → noop placeholder (needs bench-by-id wiring)
//
// The repo root that runners need defaults to ".". A future deployment
// that routes multiple PRs through one server will compute the root
// per-event from the GitHub clone path.
type realDispatcher struct {
	repoRoot string
}

func newRealDispatcher(repoRoot string) *realDispatcher {
	if repoRoot == "" {
		repoRoot = "."
	}
	return &realDispatcher{repoRoot: repoRoot}
}

// Handle implements slash.Dispatcher.
func (d *realDispatcher) Handle(ev slash.WebhookEvent, cmd *slash.Command) (string, error) {
	if cmd == nil {
		return "", fmt.Errorf("nil command")
	}
	switch cmd.Verb {
	case slash.VerbCommands:
		return slash.RenderCommandList(), nil

	case slash.VerbDismiss:
		reason := cmd.Keyword["reason"]
		if reason == "" {
			return "", fmt.Errorf("/dismiss requires reason:<text>")
		}
		// Finding-id resolution order:
		//   1. The X-Terrain-Finding-Id header (proxy-injected from
		//      the parent comment's hidden marker).
		//   2. `finding:<id>` keyword on the slash command itself
		//      (user-typed fallback when the proxy is not configured).
		findingID := ev.FindingID
		if findingID == "" {
			findingID = cmd.Keyword["finding"]
		}
		if findingID == "" {
			return "Cannot dismiss: this command must be a reply to a Terrain finding's inline comment, or include `finding:<id>` (copy the ID from the finding card).", nil
		}
		// Default scope=instance with auto content-hash; default
		// expiry per scope. Owner is the comment author.
		owner := ""
		if ev.Sender != "" {
			owner = "@" + ev.Sender
		}
		if err := runSuppress(findingID, reason, "", owner, "instance", d.repoRoot); err != nil {
			return "", fmt.Errorf("runSuppress: %w", err)
		}
		// Reset the per-repo finding-history demote state so the
		// next fire of this (rule, file) doesn't render with a stale
		// "demoted to footer" badge. Failure here is non-fatal —
		// the suppression has already been written and the user
		// gets the visible dismissal.
		if err := recordDismissInHistory(findingID, d.repoRoot); err != nil {
			// Log to stderr only; don't surface as a slash reply
			// since the suppress already succeeded.
			fmt.Fprintf(os.Stderr, "warn: finding-history dismiss failed: %v\n", err)
		}
		return fmt.Sprintf("Dismissed %s (scope=instance). Reason: %q.", findingID, reason), nil

	case slash.VerbExplain, slash.VerbWhy:
		if len(cmd.Positional) == 0 {
			return "", fmt.Errorf("/%s requires a rule-id", cmd.Verb)
		}
		ruleID := strings.Join(cmd.Positional, " ")
		verbose := cmd.Verb == slash.VerbExplain // /terrain why is the short form
		return d.captureRun(func() error {
			return runExplain(ruleID, d.repoRoot, "", false, verbose)
		})

	case slash.VerbShow:
		if len(cmd.Positional) == 0 {
			return "", fmt.Errorf("/terrain show requires an id")
		}
		id := strings.Join(cmd.Positional, " ")
		return d.captureRun(func() error {
			return runShow("finding", id, d.repoRoot, false)
		})

	case slash.VerbRefresh:
		return "_/terrain refresh acknowledged — full re-analyze + comment-edit is deferred to a future release; the existing PR check-run already re-runs on push._", nil

	case slash.VerbExpand:
		return "_/terrain expand acknowledged — inline expansion of `+N more` blocks is deferred to a future release._", nil

	case slash.VerbEscalate:
		return "_/terrain escalate acknowledged — per-PR tier override is deferred to a future release._", nil

	case slash.VerbScaffold:
		return "_/terrain scaffold accept acknowledged — test-scaffold materialization is deferred (the underlying scaffold engine lands in a later phase)._", nil

	case slash.VerbBench:
		id := strings.Join(cmd.Positional, " ")
		return fmt.Sprintf("_/terrain bench %s acknowledged — benchmark dispatch lands in a later phase._", id), nil
	}
	return fmt.Sprintf("Unhandled verb `%s`.", cmd.Verb), nil
}

// captureRun redirects the runner's stdout to a buffer for the
// duration of fn and returns the captured output as the slash reply.
// Runners like runExplain print directly to stdout; the webhook
// reply needs that text as markdown.
//
// Not safe for concurrent dispatch across goroutines (the os.Stdout
// swap is process-global). The webhook handler serializes incoming
// requests so this is fine in practice.
func (d *realDispatcher) captureRun(fn func() error) (string, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	runErr := fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	if runErr != nil {
		return "", runErr
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		out = "_(no output)_"
	}
	// Wrap the captured text in a markdown code fence so terminal
	// formatting (rules, badges) renders predictably on GitHub.
	return "```\n" + out + "\n```", nil
}

// resolveRepoRoot picks the repo root for a webhook event. For now,
// defaults to the process's cwd. Future multi-repo deployments will
// clone the PR's commit to a temp directory per-event and pass that
// path here.
func resolveRepoRoot() string {
	if cwd, err := filepath.Abs("."); err == nil {
		return cwd
	}
	return "."
}

// recordDismissInHistory writes the user's dismissal to the per-repo
// finding-history store so the renderer's demote machinery treats this
// as an active user signal (LastDismiss). Without this, the
// dismiss-overrides-fatigue-demotion contract — documented on
// findinghistory.Store.ShouldDemote — never fires in production,
// because nothing else in the CLI writes LastDismiss.
//
// Returns nil when the finding-id can't be parsed (the suppression
// path uses a different format, e.g. file-scope) or when the history
// store can't be loaded; in both cases the suppression still happened.
func recordDismissInHistory(findingID, repoRoot string) error {
	ruleID, filePath, _, _, ok := identity.ParseFindingID(findingID)
	if !ok || ruleID == "" || filePath == "" {
		return nil
	}
	store, err := engine.LoadFindingHistory(repoRoot)
	if err != nil {
		return err
	}
	store.Dismiss(ruleID, filePath)
	return store.Save(filepath.Join(repoRoot, findinghistory.DefaultPath))
}
