package remediate_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
	"github.com/pmclSF/terrain/internal/aipiperun"
	"github.com/pmclSF/terrain/internal/findingbridge"
	"github.com/pmclSF/terrain/internal/findings"
	"github.com/pmclSF/terrain/internal/remediate"
)

// writeFile writes a file under root, creating parent dirs.
func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// aiSurfaceRepo writes a minimal repo with one LLM-call surface and no eval.
func aiSurfaceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name":"app","dependencies":{"openai":"^4.0"}}`)
	writeFile(t, root, "src/handler.ts", `
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

// TestE2E_RealDetectorRemediationClosesTheLoop exercises the full path end to
// end on a REAL rule (no fake re-run): the AI pipeline detects a missing-eval
// surface, the bridge lands it on the canonical finding with a structured
// new_file Fix, and the closed-loop validator confirms applying that Fix and
// re-running the real detector clears the finding with no regressions.
//
// This confirms both claims about a finding: the finding is valid AND its
// remediation is valid.
func TestE2E_RealDetectorRemediationClosesTheLoop(t *testing.T) {
	root := aiSurfaceRepo(t)

	rerun := func(r string) ([]findings.Finding, error) {
		ai, err := aipiperun.RunRepo(context.Background(), r,
			[]string{"ai.surface.missing_eval"}, aipipeline.PostureObservability)
		if err != nil {
			return nil, err
		}
		out := make([]findings.Finding, 0, len(ai))
		for _, f := range ai {
			out = append(out, findingbridge.FromAIPipeline(f, ""))
		}
		return out, nil
	}

	before, err := rerun(root)
	if err != nil {
		t.Fatalf("initial detect: %v", err)
	}

	var target *findings.Finding
	for i := range before {
		if before[i].PrimaryLoc.Path == "src/handler.ts" {
			target = &before[i]
			break
		}
	}
	if target == nil {
		t.Fatal("expected a missing-eval finding for src/handler.ts")
	}
	// The bridged finding must carry the structured, applicable remediation.
	if remediate.Key(*target) == "" || target.Suggestions == nil || target.Suggestions[0].Fix == nil {
		t.Fatalf("finding must carry a structured Fix; got %+v", target.Suggestions)
	}
	if target.Suggestions[0].Fix.Kind != findings.FixNewFile {
		t.Errorf("Fix.Kind = %q, want new_file", target.Suggestions[0].Fix.Kind)
	}

	v, err := remediate.Validate(root, *target, before, rerun)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !v.Valid {
		t.Errorf("remediation should close the loop on the real detector; verdict: %s (new findings: %d)",
			v.Note, len(v.NewFindings))
	}
}
