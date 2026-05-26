package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/slash"
	"github.com/pmclSF/terrain/internal/suppression"
)

// TestRealDispatcher_DismissWritesSuppression is the end-to-end
// integration test for the Phase 5 slash-receiver → CLI runner →
// suppression-file write chain.
//
// The contract: a /dismiss webhook delivery causes a real
// .terrain/suppressions.yaml to land on disk, with the comment
// author recorded as owner. No mocks — runSuppress is the same
// runner the CLI uses.
func TestRealDispatcher_DismissWritesSuppression(t *testing.T) {
	root := t.TempDir()
	d := newRealDispatcher(root)

	ev := slash.WebhookEvent{
		Sender:    "octocat",
		FindingID: "weakAssertion@internal/auth/login_test.go:TestLogin#abc123",
	}
	cmd := &slash.Command{
		Verb:    slash.VerbDismiss,
		Keyword: map[string]string{"reason": "false positive — sanitized upstream"},
	}

	reply, err := d.Handle(ev, cmd)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(reply, "Dismissed") {
		t.Errorf("reply should confirm dismissal; got: %q", reply)
	}

	res, err := suppression.Load(filepath.Join(root, suppression.DefaultPath))
	if err != nil {
		t.Fatalf("load suppressions: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(res.Entries), res.Entries)
	}
	if !strings.Contains(res.Entries[0].Owner, "octocat") {
		t.Errorf("owner should record comment author; got %q", res.Entries[0].Owner)
	}
}

// TestRealDispatcher_DismissRequiresReason proves the receiver
// enforces the reason: keyword. Reason-less dismisses are a known
// adopter foot-gun (silent suppression with no audit trail).
func TestRealDispatcher_DismissRequiresReason(t *testing.T) {
	d := newRealDispatcher(t.TempDir())
	ev := slash.WebhookEvent{FindingID: "x@y:z#1"}
	cmd := &slash.Command{Verb: slash.VerbDismiss}
	if _, err := d.Handle(ev, cmd); err == nil {
		t.Error("expected error when reason: is missing")
	}
}

// TestRealDispatcher_DismissOutsideThreadDeclines covers the
// "comment isn't a reply to a Terrain finding" case. The receiver
// MUST NOT write a suppression without an attached finding-id —
// otherwise the next `analyze` re-emits the same finding and the
// user thinks /dismiss is broken.
func TestRealDispatcher_DismissOutsideThreadDeclines(t *testing.T) {
	d := newRealDispatcher(t.TempDir())
	ev := slash.WebhookEvent{Sender: "octocat"} // no FindingID
	cmd := &slash.Command{
		Verb:    slash.VerbDismiss,
		Keyword: map[string]string{"reason": "x"},
	}
	reply, err := d.Handle(ev, cmd)
	if err != nil {
		t.Fatalf("expected graceful decline, got error: %v", err)
	}
	if !strings.Contains(reply, "Cannot dismiss") {
		t.Errorf("reply should explain the decline; got: %q", reply)
	}
}

// TestRealDispatcher_DeferredVerbsReturnPlaceholderText proves the
// five "acknowledged, deferred to future release" verbs return a
// stable user-visible message rather than crashing. Adopters wiring
// up the receiver shouldn't see a 500 just because they invoked a
// not-yet-implemented verb.
func TestRealDispatcher_DeferredVerbsReturnPlaceholderText(t *testing.T) {
	d := newRealDispatcher(t.TempDir())
	cases := []slash.Verb{
		slash.VerbRefresh,
		slash.VerbExpand,
		slash.VerbEscalate,
		slash.VerbScaffold,
		slash.VerbBench,
	}
	for _, v := range cases {
		t.Run(string(v), func(t *testing.T) {
			cmd := &slash.Command{Verb: v}
			reply, err := d.Handle(slash.WebhookEvent{}, cmd)
			if err != nil {
				t.Fatalf("Handle(%s): %v", v, err)
			}
			if !strings.Contains(reply, "acknowledged") {
				t.Errorf("verb %s reply should mention 'acknowledged'; got: %q", v, reply)
			}
		})
	}
}

// TestRealDispatcher_CommandsListIsRendered guards the /terrain
// commands surface. Adopters use this to discover what the bot can
// do. If the list ever drifts to empty / panic, /terrain commands
// would be the canary.
func TestRealDispatcher_CommandsListIsRendered(t *testing.T) {
	d := newRealDispatcher(t.TempDir())
	cmd := &slash.Command{Verb: slash.VerbCommands}
	reply, err := d.Handle(slash.WebhookEvent{}, cmd)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// Every shipping verb must appear in the rendered list.
	for _, must := range []string{"/dismiss", "/terrain explain", "/terrain show"} {
		if !strings.Contains(reply, must) {
			t.Errorf("commands list missing %q; got:\n%s", must, reply)
		}
	}
}
