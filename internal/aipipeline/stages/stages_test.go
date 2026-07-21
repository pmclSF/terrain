package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

func TestPathPrefilter_HardDropsExamples(t *testing.T) {
	t.Parallel()
	s := NewPathPrefilter()
	c := &aipipeline.Candidate{Path: "examples/agents/demo.py", Lang: "python"}
	res := s.Run(context.Background(), c)
	if res.Continue {
		t.Errorf("examples/ path should hard-drop")
	}
	if !hasAtom(c, "path.examples") {
		t.Errorf("expected path.examples atom")
	}
}

func TestPathPrefilter_HardDropsTests(t *testing.T) {
	t.Parallel()
	s := NewPathPrefilter()
	c := &aipipeline.Candidate{Path: "tests/test_handler.py", Lang: "python"}
	res := s.Run(context.Background(), c)
	if res.Continue {
		t.Errorf("tests/ path should hard-drop")
	}
}

func TestPathPrefilter_EmitsProviderSoftSignal(t *testing.T) {
	t.Parallel()
	s := NewPathPrefilter()
	c := &aipipeline.Candidate{Path: "src/llm/providers/openai.py", Lang: "python"}
	res := s.Run(context.Background(), c)
	if !res.Continue {
		t.Errorf("provider path should not hard-drop (only soft-negative)")
	}
	if !hasAtom(c, "path.providers") {
		t.Errorf("expected path.providers atom")
	}
}

func TestRegexFastscan_OpenAIPythonChat(t *testing.T) {
	t.Parallel()
	src := []byte(`import openai

client = openai.OpenAI(api_key="x")
resp = client.chat.completions.create(model="gpt-4o", messages=[{}])
`)
	c := &aipipeline.Candidate{Path: "app/handler.py", Lang: "python", Src: src}
	r := NewRegexFastscan()
	res := r.Run(context.Background(), c)
	if !res.Continue {
		t.Fatalf("regex stage should not drop candidates")
	}
	if !hasAtom(c, "regex.openai.import") {
		t.Errorf("expected regex.openai.import atom on file with `import openai`")
	}
	if !hasAtom(c, "regex.openai.call") {
		t.Errorf("expected regex.openai.call atom on chat.completions.create")
	}
}

func TestRegexFastscan_AnthropicTypeScript(t *testing.T) {
	t.Parallel()
	src := []byte(`import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic();
const msg = await client.messages.create({ model: "claude-3-5", messages: [] });
`)
	c := &aipipeline.Candidate{Path: "src/api/chat.ts", Lang: "typescript", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "regex.anthropic.import") {
		t.Errorf("expected anthropic import atom for @anthropic-ai/sdk")
	}
	if !hasAtom(c, "regex.anthropic.call") {
		t.Errorf("expected anthropic call atom for .messages.create")
	}
}

func TestRegexFastscan_LangChainJSImport(t *testing.T) {
	t.Parallel()
	src := []byte(`import { ChatOpenAI } from "@langchain/openai";
import { StateGraph } from "@langchain/langgraph";

const llm = new ChatOpenAI();
const result = await llm.invoke([{ role: "user", content: "hi" }]);
`)
	c := &aipipeline.Candidate{Path: "src/chains/qa.ts", Lang: "typescript", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "regex.langchain.import") {
		t.Errorf("expected langchain.import atom")
	}
	if !hasAtom(c, "regex.langchain.call") {
		t.Errorf("expected langchain.call atom on llm.invoke")
	}
}

func TestRegexFastscan_GoOpenAI(t *testing.T) {
	t.Parallel()
	src := []byte(`package main

import (
	"context"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	c := openai.NewClient("xxx")
	c.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{})
}
`)
	c := &aipipeline.Candidate{Path: "cmd/agent/main.go", Lang: "go", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "regex.openai.import") {
		t.Errorf("expected openai.import atom for go-openai")
	}
	if !hasAtom(c, "regex.openai.call") {
		t.Errorf("expected openai.call atom for CreateChatCompletion")
	}
}

func TestRegexFastscan_ImportOnlyEmitsNegative(t *testing.T) {
	t.Parallel()
	// A file that imports langchain types but never calls an LLM.
	src := []byte(`from langchain.schema import BaseMessage

class MessageFactory:
    def make(self) -> BaseMessage:
        return BaseMessage(content="hi", type="system")
`)
	c := &aipipeline.Candidate{Path: "app/factories/message.py", Lang: "python", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "regex.langchain.import") {
		t.Errorf("expected langchain.import atom")
	}
	if hasAtom(c, "regex.langchain.call") {
		t.Errorf("did not expect langchain.call atom; the file just imports types")
	}
	if !hasAtom(c, "regex.import_without_call") {
		t.Errorf("expected the regex-derived negative gate to fire when import without call")
	}
}

func TestRegexFastscan_WrapperClassEmitsNegative(t *testing.T) {
	t.Parallel()
	// A class that wraps an LLM provider — defines the canonical
	// wrapper methods but doesn't call openai at module level.
	src := []byte(`from openai import OpenAI

class OpenAIProvider(LLM):
    def __init__(self, api_key):
        self.client = OpenAI(api_key=api_key)

    def chat(self, prompt):
        return self.client.chat.completions.create(model="gpt-4o", messages=[])

    def stream(self, prompt):
        yield from []
`)
	c := &aipipeline.Candidate{Path: "src/llm/openai_provider.py", Lang: "python", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "wrapper.class.match") {
		t.Errorf("expected wrapper.class.match atom on provider-shaped class")
	}
}

func TestChangeScope_DiffTouchedFile(t *testing.T) {
	t.Parallel()
	s := NewChangeScope()
	c := &aipipeline.Candidate{
		Path:   "app/handler.py",
		Lang:   "python",
		RuleID: "ai.surface.missing_eval",
		Diff: &aipipeline.DiffContext{
			TouchedFiles: map[string]map[int]struct{}{
				"app/handler.py": {12: {}},
			},
		},
		Atoms: []aipipeline.EvidenceAtom{
			{RuleID: "regex.openai.call", Span: aipipeline.Span{Line: 12}},
		},
	}
	s.Run(context.Background(), c)
	if !hasAtom(c, "scope.diff_touched_file") {
		t.Errorf("expected scope.diff_touched_file atom")
	}
	if !hasAtom(c, "scope.diff_touched_line") {
		t.Errorf("expected scope.diff_touched_line atom when atom span matches diff line")
	}
}

func TestChangeScope_NoDiffNoAtoms(t *testing.T) {
	t.Parallel()
	s := NewChangeScope()
	c := &aipipeline.Candidate{Path: "x.py", Diff: nil}
	s.Run(context.Background(), c)
	if hasAtom(c, "scope.diff_touched_file") {
		t.Errorf("expected no scope atoms when Diff is nil")
	}
}

func TestChangeScope_PRRemediationAtom(t *testing.T) {
	t.Parallel()
	c := &aipipeline.Candidate{Path: "x.py", Diff: &aipipeline.DiffContext{}}
	AddPRRemediation(c, "added evals/qa.yaml")
	if !hasAtom(c, "scope.diff_added_pr_evidence") {
		t.Errorf("expected pr-remediation atom")
	}
}

func TestRegexFastscan_ProductionMLSDKEmits(t *testing.T) {
	t.Parallel()
	src := []byte(`import sagemaker
from sklearn.ensemble import RandomForestClassifier
clf = RandomForestClassifier()
clf.fit(X_train, y_train)
`)
	c := &aipipeline.Candidate{Path: "pipelines/train.py", Lang: "python", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "regex.production_ml_sdk") {
		t.Errorf("expected regex.production_ml_sdk on sagemaker import")
	}
	if !hasAtom(c, "regex.sklearn_train.call") {
		t.Errorf("training atom should still fire alongside production-context atom")
	}
}

func TestRegexFastscan_NoProductionContextOnPureSklearn(t *testing.T) {
	t.Parallel()
	// Tutorial-shaped: only sklearn, no production-context signals.
	src := []byte(`from sklearn.ensemble import RandomForestClassifier
clf = RandomForestClassifier()
clf.fit(X_train, y_train)
`)
	c := &aipipeline.Candidate{Path: "notebook_export.py", Lang: "python", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if hasAtom(c, "regex.production_ml_sdk") {
		t.Errorf("must not fire production_ml_sdk without an actual production import")
	}
	if hasAtom(c, "regex.scheduling_decorator") {
		t.Errorf("must not fire scheduling_decorator without an actual decorator")
	}
}

func TestRegexFastscan_AirflowDecoratorEmits(t *testing.T) {
	t.Parallel()
	src := []byte(`from airflow.decorators import task
from sklearn.ensemble import RandomForestClassifier

@task
def train():
    clf = RandomForestClassifier()
    clf.fit(X_train, y_train)
`)
	c := &aipipeline.Candidate{Path: "dags/train_model.py", Lang: "python", Src: src}
	r := NewRegexFastscan()
	r.Run(context.Background(), c)
	if !hasAtom(c, "regex.scheduling_decorator") {
		t.Errorf("expected regex.scheduling_decorator on @task")
	}
}

func TestCrossFileScope_NilResolverNoAtoms(t *testing.T) {
	t.Parallel()
	s := NewCrossFileScope(nil)
	c := &aipipeline.Candidate{Path: "app/handler.py"}
	s.Run(context.Background(), c)
	if len(c.Atoms) != 0 {
		t.Errorf("nil resolver must produce zero atoms (corpus harness depends on this)")
	}
}

func TestCrossFileScope_SiblingEvalSuppresses(t *testing.T) {
	t.Parallel()
	s := NewCrossFileScope(&fakeResolver{sibling: true})
	c := &aipipeline.Candidate{Path: "app/handler.py"}
	s.Run(context.Background(), c)
	if !hasAtom(c, "scope.sibling_has_eval") {
		t.Errorf("expected sibling_has_eval atom when resolver reports sibling marker")
	}
	if hasAtom(c, "scope.package_has_eval") {
		t.Errorf("did not expect package atom; sibling already matched")
	}
}

func TestCrossFileScope_PackageFallback(t *testing.T) {
	t.Parallel()
	s := NewCrossFileScope(&fakeResolver{pkg: true})
	c := &aipipeline.Candidate{Path: "app/handler.py"}
	s.Run(context.Background(), c)
	if !hasAtom(c, "scope.package_has_eval") {
		t.Errorf("expected package_has_eval atom when only package matched")
	}
}

type fakeResolver struct {
	sibling    bool
	pkg        bool
	referenced bool
}

func (f *fakeResolver) SiblingHasEvalMarker(string) bool    { return f.sibling }
func (f *fakeResolver) PackageHasEvalMarker(string) bool    { return f.pkg }
func (f *fakeResolver) SurfaceReferencedByEval(string) bool { return f.referenced }

// helper: searches the candidate's atoms for one with the given RuleID.
func hasAtom(c *aipipeline.Candidate, ruleID string) bool {
	for _, a := range c.Atoms {
		if a.RuleID == ruleID {
			return true
		}
	}
	return false
}

// helper: returns the first atom matching the predicate (debugging aid).
func _findAtom(c *aipipeline.Candidate, contains string) *aipipeline.EvidenceAtom { //nolint:unused
	for i, a := range c.Atoms {
		if strings.Contains(a.RuleID, contains) {
			return &c.Atoms[i]
		}
	}
	return nil
}
