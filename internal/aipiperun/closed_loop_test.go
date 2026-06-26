package aipiperun

import (
	"context"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// fixtureAISurfaceRepo writes a minimal app-shaped repo with one LLM-call
// surface and no eval, returning the root.
func fixtureAISurfaceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkfile(t, root, "package.json", `{"name":"app","dependencies":{"openai":"^4.0"}}`)
	mkfile(t, root, "src/handler.ts", `
import OpenAI from "openai";
const client = new OpenAI();
export async function ask(q: string) {
  const r = await client.chat.completions.create({
    model: "gpt-4o",
    messages: [{ role: "user", content: q }],
  });
  return r.choices[0].message.content;
}
`)
	return root
}

func handlerFinding(findings []aipipeline.Finding) *aipipeline.Finding {
	for i := range findings {
		if findings[i].Path == "src/handler.ts" {
			return &findings[i]
		}
	}
	return nil
}

// TestRunRepo_FindingCarriesMaterializableScaffold proves the "useful"
// half of the closed loop: a missing-eval finding now ships its own
// runnable protection patch (the scaffold body) plus the path to write
// it — so the finding is actionable, not merely diagnostic.
func TestRunRepo_FindingCarriesMaterializableScaffold(t *testing.T) {
	t.Parallel()
	root := fixtureAISurfaceRepo(t)

	findings, err := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"}, aipipeline.PostureObservability)
	if err != nil {
		t.Fatalf("RunRepo: %v", err)
	}
	fnd := handlerFinding(findings)
	if fnd == nil {
		t.Fatal("expected a missing-eval finding for src/handler.ts")
	}
	// Exact target path, derived from the surface basename — the scaffold
	// must land where coverage discovery will look.
	if fnd.FixScaffoldPath != "evals/promptfoo/handler.yaml" {
		t.Errorf("FixScaffoldPath = %q, want evals/promptfoo/handler.yaml", fnd.FixScaffoldPath)
	}
	// The scaffold must reference the surface it protects — this is the
	// precondition the closure (re-analyze resolves the finding) relies on.
	// A non-empty but surface-less scaffold would be useless, so assert the
	// reference, not merely non-emptiness.
	if !strings.Contains(fnd.FixScaffold, "src/handler.ts") {
		t.Errorf("scaffold body should reference surface src/handler.ts; got:\n%s", fnd.FixScaffold)
	}
}

// TestRunRepo_ScaffoldClosesTheLoop is the "trustworthy" half of the closed
// loop: after the scaffold is materialized — a promptfoo eval under
// evals/promptfoo/ that references the surface via file://<path> — a
// re-analyze RESOLVES the finding, because the protection now exists.
func TestRunRepo_ScaffoldClosesTheLoop(t *testing.T) {
	root := fixtureAISurfaceRepo(t)
	findings, _ := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"}, aipipeline.PostureObservability)
	fnd := handlerFinding(findings)
	if fnd == nil || fnd.FixScaffoldPath == "" {
		t.Fatal("setup: expected finding with a scaffold path")
	}

	mkfile(t, root, fnd.FixScaffoldPath, fnd.FixScaffold)

	findings2, _ := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"}, aipipeline.PostureObservability)
	if handlerFinding(findings2) != nil {
		t.Errorf("finding should resolve after materializing the scaffold at %s", fnd.FixScaffoldPath)
	}
}

// TestRunRepo_UnrelatedEvalDoesNotClose pins the PRECISION half of the
// contract: an eval that exists under evals/ but references a DIFFERENT
// surface must NOT suppress this finding. Closure links by explicit path
// reference, never broadly by "an eval exists somewhere".
func TestRunRepo_UnrelatedEvalDoesNotClose(t *testing.T) {
	t.Parallel()
	root := fixtureAISurfaceRepo(t) // src/handler.ts, no eval covering it
	mkfile(t, root, "evals/promptfoo/other.yaml",
		"description: other\nprompts:\n  - file://src/other.ts\n")
	findings, _ := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"}, aipipeline.PostureObservability)
	if handlerFinding(findings) == nil {
		t.Error("an eval referencing a DIFFERENT surface must not suppress handler.ts's finding")
	}
}
