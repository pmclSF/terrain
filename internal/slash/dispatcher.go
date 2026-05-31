package slash

import (
	"fmt"
	"strings"
)

// DefaultDispatcher is a zero-config Dispatcher useful for local
// testing and for the `terrain serve --webhook` mode. It produces a
// markdown reply that describes what the verb would do, without
// actually mutating state or running the underlying analysis.
//
// Production deployments override this with a Dispatcher that's
// wired to the existing CLI runners (runShow, runExplain,
// runSuppress) plus a GitHub-API client that posts the reply back
// to the PR.
//
// The intent: ship a working webhook receiver so adopters can
// validate the wiring end-to-end; the dispatch table they override
// is the only piece they need to provide.
type DefaultDispatcher struct{}

// Handle implements Dispatcher. Returns a markdown reply describing
// what the verb would do.
func (DefaultDispatcher) Handle(ev WebhookEvent, cmd *Command) (string, error) {
	if cmd == nil {
		return "", fmt.Errorf("nil command")
	}
	switch cmd.Verb {
	case VerbCommands:
		return RenderCommandList(), nil
	case VerbDismiss:
		reason := cmd.Keyword["reason"]
		return fmt.Sprintf("`/dismiss` recorded with reason: %q. Wire a Dispatcher that calls into `internal/suppression` to materialize the entry.", reason), nil
	case VerbExplain, VerbWhy:
		ruleID := strings.Join(cmd.Positional, " ")
		return fmt.Sprintf("`/%s %s` — wire a Dispatcher that consults `internal/severity/rubric` and renders the rule's stable clause IDs.", cmd.Verb, ruleID), nil
	case VerbShow:
		id := strings.Join(cmd.Positional, " ")
		return fmt.Sprintf("`/terrain show %s` — wire a Dispatcher that runs `runReportShow` against the latest snapshot.", id), nil
	case VerbExpand:
		return "`/terrain expand` — wire a Dispatcher that re-renders the collapsed `+N more` block inline.", nil
	case VerbRefresh:
		return "`/terrain refresh` — wire a Dispatcher that re-runs analyze and edits the pinned comment.", nil
	case VerbEscalate:
		return "`/terrain escalate` — wire a Dispatcher that flips the active finding's tier for this PR only.", nil
	case VerbScaffold:
		return "`/terrain scaffold accept` — wire a Dispatcher that materializes the test scaffold the finding suggested.", nil
	case VerbBench:
		id := strings.Join(cmd.Positional, " ")
		return fmt.Sprintf("`/terrain bench %s` — wire a Dispatcher that runs the benchmark suite by id.", id), nil
	}
	return fmt.Sprintf("Unhandled verb `%s` — extend DefaultDispatcher.Handle().", cmd.Verb), nil
}
