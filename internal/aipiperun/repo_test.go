package aipiperun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

func TestRunRepo_EmitsOnAppShapedRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Mimic an AI-feature-in-app repo: production-shaped source tree
	// with an LLM call but no sibling eval.
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
	findings, err := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"},
		aipipeline.PostureObservability)
	if err != nil {
		t.Fatalf("RunRepo: %v", err)
	}
	if len(findings) == 0 {
		t.Errorf("expected at least one finding on a missing-eval handler; got none")
	}
}

func TestRunRepo_SuppressesWhenSiblingHasEval(t *testing.T) {
	t.Parallel()
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
	// Sibling file imports an eval framework — cross-file Stage 4
	// should fire scope.sibling_has_eval and tank the verdict.
	mkfile(t, root, "src/handler.test.ts", `
import { describe, it, expect } from "vitest";
import { ask } from "./handler";
describe("ask", () => {
  it("answers", async () => { expect(await ask("hi")).toBeTruthy(); });
});
`)
	findings, err := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"},
		aipipeline.PostureObservability)
	if err != nil {
		t.Fatalf("RunRepo: %v", err)
	}
	for _, f := range findings {
		if f.Path == "src/handler.ts" {
			t.Errorf("handler.ts should not emit when sibling vitest test exists; got conf=%.3f atoms=%+v",
				f.Confidence, f.Atoms)
		}
	}
}

func TestRunRepo_SkipsNodeModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "package.json", `{}`)
	mkfile(t, root, "node_modules/openai/index.js",
		`import OpenAI from "openai"; const c = new OpenAI(); c.chat.completions.create({});`)
	findings, _ := RunRepo(context.Background(), root,
		[]string{"ai.surface.missing_eval"},
		aipipeline.PostureObservability)
	for _, f := range findings {
		if filepath.Dir(f.Path) == "node_modules/openai" {
			t.Errorf("node_modules must be skipped; got finding at %s", f.Path)
		}
	}
}

func TestRunRepo_NoRulesReturnsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "src/handler.py", "import openai\nopenai.chat.completions.create()\n")
	findings, err := RunRepo(context.Background(), root, nil, aipipeline.PostureObservability)
	if err != nil {
		t.Fatalf("RunRepo: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected zero findings when no rules supplied; got %d", len(findings))
	}
}

func mkfile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
