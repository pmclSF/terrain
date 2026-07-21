package slash

import "strings"

// RenderCommandList produces the markdown for the pinned
// command-list comment Terrain posts on first interaction with a PR.
// The comment stays pinned (re-rendered into the same comment ID on
// every PR-comment hook) so users always know which verbs are
// available.
//
// Pinned at first-time bot interaction as a top-level command-list
// comment so the verb surface is always discoverable.
//
// The output is stable across calls (no timestamps, no randomized
// ordering) so a re-render produces byte-identical text — the
// webhook handler can compare and skip the edit when there's no
// change.
func RenderCommandList() string {
	var b strings.Builder
	b.WriteString("**Terrain — slash commands**\n\n")
	b.WriteString("Reply to any inline finding with one of:\n\n")
	for _, line := range commandLines() {
		b.WriteString("- `")
		b.WriteString(line.Syntax)
		b.WriteString("` — ")
		b.WriteString(line.Help)
		b.WriteString("\n")
	}
	b.WriteString("\n_Commands are case-insensitive. Quoted values support spaces. ")
	b.WriteString("All actions are deterministic — no LLM parsing._\n")
	return b.String()
}

type commandHelp struct {
	Syntax string
	Help   string
}

func commandLines() []commandHelp {
	return []commandHelp{
		{"/dismiss reason:<text>", "Suppress this finding. Adds an entry to .terrain/suppressions.yaml with content_hash so the suppression invalidates if the line changes."},
		{"/terrain explain <rule-id>", "Long-form explanation of a rule (what it catches, why, how to fix)."},
		{"/terrain why <rule-id>", "One-paragraph rationale for why a rule exists."},
		{"/terrain show <id>", "Render one finding by its ID."},
		{"/terrain expand", "Show the full findings list behind a collapsed `+N more` block."},
		{"/terrain refresh", "Re-run analyze and update this PR comment. (Planned — not yet active.)"},
		{"/terrain escalate", "Promote a finding from observability to gate for this PR only. (Planned — not yet active.)"},
		{"/terrain scaffold accept", "Accept the test scaffold this finding suggested."},
		{"/terrain bench <id>", "Run a benchmark suite by ID against this PR. (Planned — not yet active; run locally with `terrain ai run`.)"},
		{"/terrain commands", "Re-print this command list."},
	}
}
