package stages

import (
	"context"
	"regexp"
	"strings"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

// RegexFastscan is Stage 2: context-window regex scanning. It emits
// lexical atoms when an SDK anchor and a call-form verb co-occur in
// the same file. Uses a context-loose composition with explicit
// negative atoms for shape suppression.
//
// Two derived atoms:
//
//	regex.<sdk>.import — anchor matched (an LLM SDK is imported)
//	regex.<sdk>.call   — a call-form verb matched in the file
//
// Plus a separate "wrapper-class" atom when the file's shape looks
// like a provider/adapter (class inherits from LLM-style base + defines
// canonical wrapper methods, no module-level call). That atom is
// scope-aware via has_module_level_call.
//
// Critically, this stage may emit "ast.no_call_despite_regex" as a
// negative atom when the SDK anchor matched but no real call site
// was found — the regex-v2 "negative gate" effect. (The AST stage
// can emit a stronger version of this atom; this stage produces a
// regex-only approximation.)
type RegexFastscan struct {
	// MaxLineLen caps the length of each line read into the regex
	// engine. Lines longer than this are truncated for matching.
	MaxLineLen int

	// MaxFileBytes is a hard upper bound. Files larger than this skip
	// the verb regex pass and emit a fallback marker. 0 disables.
	MaxFileBytes int
}

// NewRegexFastscan returns a fastscan stage with sensible defaults.
func NewRegexFastscan() *RegexFastscan {
	return &RegexFastscan{
		MaxLineLen:   4096,
		MaxFileBytes: 2 * 1024 * 1024, // 2 MB
	}
}

// Name implements pipeline.Stage.
func (s *RegexFastscan) Name() string { return "regex-fastscan" }

// Run scans the candidate's Src content, matching anchor/verb pairs
// and emitting atoms.
func (s *RegexFastscan) Run(_ context.Context, c *aipipeline.Candidate) aipipeline.StageResult {
	if len(c.Src) == 0 {
		c.AddFallback("source-unavailable")
		return aipipeline.StageResult{Continue: true}
	}
	if s.MaxFileBytes > 0 && len(c.Src) > s.MaxFileBytes {
		c.AddFallback("source-too-large")
		return aipipeline.StageResult{Continue: true}
	}

	matchedAnchor := false
	matchedVerb := false
	for _, pair := range ctxPairs {
		anchorHit := pair.anchor.MatchString(string(c.Src))
		if !anchorHit {
			continue
		}
		matchedAnchor = true
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceLexical,
			RuleID: "regex." + pair.name + ".import",
			Source: "regex-fastscan",
			Weight: defaultImportWeight(pair.name),
			Span:   aipipeline.Span{Snippet: pair.name + " import"},
		})
		if pair.verb.MatchString(string(c.Src)) {
			matchedVerb = true
			c.AddAtom(aipipeline.EvidenceAtom{
				Kind:   aipipeline.EvidenceLexical,
				RuleID: "regex." + pair.name + ".call",
				Source: "regex-fastscan",
				Weight: defaultCallWeight(pair.name),
				Span:   aipipeline.Span{Snippet: pair.name + " call"},
			})
		}
	}

	// Loose anchor: any known SDK import. Used to compose the
	// negative-gate when no verb fires.
	if !matchedAnchor && sdkPresentRE.MatchString(string(c.Src)) {
		matchedAnchor = true
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceLexical,
			RuleID: "regex.generic_sdk.import",
			Source: "regex-fastscan",
			Weight: +0.2,
			Span:   aipipeline.Span{Snippet: "generic SDK import"},
		})
	}

	// Loose verb fallback when context-window pairs didn't fire but a
	// known call shape exists.
	if matchedAnchor && !matchedVerb && looseVerbRE.MatchString(string(c.Src)) {
		matchedVerb = true
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceLexical,
			RuleID: "regex.generic_sdk.call",
			Source: "regex-fastscan",
			Weight: +1.0,
			Span:   aipipeline.Span{Snippet: "loose call"},
		})
	}

	// Negative gate: SDK import present, no call found anywhere.
	// Distinct atom ID from the AST-derived "ast.no_call_despite_regex"
	// so the composer doesn't double-count when both stages agree.
	if matchedAnchor && !matchedVerb {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "regex.import_without_call",
			Source: "regex-fastscan",
			Weight: -1.6, // regex-derived; AST atom is the stronger version
			Span:   aipipeline.Span{Snippet: "import-without-call"},
		})
	}

	// Wrapper-class detection — files that wrap a provider but don't
	// invoke it at module scope.
	if matchedAnchor && isWrapperFile(c.Src) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "wrapper.class.match",
			Source: "regex-fastscan",
			Weight: -2.0,
			Span:   aipipeline.Span{Snippet: "provider wrapper class"},
		})
	}

	// Multi-framework heuristic: when 3+ distinct ML-training
	// frameworks are imported in the same file, this is almost
	// certainly library code that supports many backends — not an
	// application training pipeline that just happens to mix them.
	// Empirical false-positive cluster.
	if mlTrainingFrameworkCount(c.Src) >= 3 {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "regex.multi_framework",
			Source: "regex-fastscan",
			Weight: -2.0,
			Span:   aipipeline.Span{Snippet: "3+ ML-training frameworks imported"},
		})
	}

	// Provider-integration directory + AST confirms: an extra
	// negative atom that fires when the file path lives in a
	// framework's integration tree (/integrations/llms/, /llms/<X>/,
	// /providers/, etc.) AND the regex saw a real call. These are
	// genuine LLM call sites — but in framework code, where evals
	// belong at the application layer, not the integration.
	if matchedVerb && isFrameworkIntegrationPath(c.Path) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceNegative,
			RuleID: "path.framework_integration",
			Source: "regex-fastscan",
			Weight: -2.5,
			Span:   aipipeline.Span{Snippet: c.Path},
		})
	}

	// Production-context atoms — fire when the file shows signals
	// that distinguish production training/serving from research or
	// tutorial code. These atoms carry meaningful weight only for
	// training rules (calibrated separately); the surface rule treats
	// them as neutral.
	if productionMLSDKRE.Match(c.Src) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceLexical,
			RuleID: "regex.production_ml_sdk",
			Source: "regex-fastscan",
			Weight: +1.5,
			Span:   aipipeline.Span{Snippet: "production ML SDK import"},
		})
	}
	if hasSchedulingDecorator(c.Src) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceLexical,
			RuleID: "regex.scheduling_decorator",
			Source: "regex-fastscan",
			Weight: +1.5,
			Span:   aipipeline.Span{Snippet: "@airflow / @prefect / @dagster / @ray decorator"},
		})
	}
	if modelRegistryRE.Match(c.Src) {
		c.AddAtom(aipipeline.EvidenceAtom{
			Kind:   aipipeline.EvidenceLexical,
			RuleID: "regex.model_registry_register",
			Source: "regex-fastscan",
			Weight: +1.2,
			Span:   aipipeline.Span{Snippet: "model registry / artifact-store call"},
		})
	}

	return aipipeline.StageResult{Continue: true}
}

// Production-context regexes — anchored on production ML/serving
// signals. A training-anchored file with one of these is plausibly
// production code; without them, it's far more likely research,
// tutorial, or kaggle export and the missing-tracker finding isn't
// actionable.
var (
	// productionMLSDKRE matches imports of frameworks whose presence
	// is hard to explain outside of a deployment context. Ray and
	// pure-research frameworks (metaflow, kedro) are intentionally
	// excluded — they show up in research code too.
	productionMLSDKRE = regexp.MustCompile(
		`(?m)^\s*(?:from|import)\s+(?:` +
			`sagemaker|azureml|google\.cloud\.aiplatform|vertexai|` +
			`mlflow\.deployments|mlflow\.sagemaker|` +
			`bentoml|kfserving|seldon|kserve|` +
			`torchserve|triton` +
			`)\b`)
	// schedulingDecoratorPrefixedRE matches namespaced decorator calls
	// where the framework is on the left: @airflow.task, @prefect.flow.
	// These are unambiguous.
	schedulingDecoratorPrefixedRE = regexp.MustCompile(
		`(?m)^\s*@(?:airflow|prefect|dagster|dlt|ray|metaflow|kedro)\b`)
	// schedulingFrameworkImportRE matches an import of a scheduling
	// framework so we can promote bare decorators (@task, @flow, etc.)
	// in the same file with confidence.
	schedulingFrameworkImportRE = regexp.MustCompile(
		`(?m)^\s*(?:from|import)\s+(?:airflow|prefect|dagster|dlt|ray|metaflow|kedro)\b`)
	// bareSchedulingDecoratorRE matches the decorator names that are
	// idiomatic for scheduling frameworks. We require the corresponding
	// framework import to be present in the same file.
	bareSchedulingDecoratorRE = regexp.MustCompile(
		`(?m)^\s*@(?:task|flow|asset|dag|pipeline|step)\b`)
	// modelRegistryRE matches model-registry calls anchored on the
	// hosting framework (mlflow / bentoml / wandb). Bare names like
	// register_model() or .save_model() are excluded — those false-
	// fire on framework source code (xgboost / RayDP / shorttext)
	// that defines methods with the same names.
	modelRegistryRE = regexp.MustCompile(
		`mlflow\.register_model\s*\(` +
			`|mlflow\.log_model\s*\(` +
			`|wandb\.Artifact\s*\(\s*['"][^'"]+['"]\s*,\s*type\s*=\s*['"]model['"]` +
			`|bentoml\.save_model\s*\(` +
			`|bentoml\.models\.create\s*\(`)
)

// hasSchedulingDecorator reports whether the source carries a
// scheduling-framework decorator. Two ways it fires:
//
//  1. Namespaced decorator (@airflow.task, @prefect.flow, ...) — always
//     confirms scheduling intent.
//  2. Bare decorator (@task, @flow, @asset, ...) accompanied by an
//     import of the relevant framework. The bare form is idiomatic
//     for airflow's `from airflow.decorators import task` pattern and
//     prefect's `from prefect import flow`; we require the framework
//     import so @task on an unrelated class method doesn't false-fire.
func hasSchedulingDecorator(src []byte) bool {
	if schedulingDecoratorPrefixedRE.Match(src) {
		return true
	}
	if bareSchedulingDecoratorRE.Match(src) && schedulingFrameworkImportRE.Match(src) {
		return true
	}
	return false
}

// mlTrainingFrameworkCount returns the number of distinct ML-training
// frameworks imported by the file. A high count is a library signal.
func mlTrainingFrameworkCount(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	frameworks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s*(?:from|import)\s+sklearn\b`),
		regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:xgboost|lightgbm|catboost)\b`),
		regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:tensorflow|keras)\b`),
		regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:torch|pytorch_lightning|lightning|pl)\b`),
		regexp.MustCompile(`(?m)^\s*from\s+transformers\b`),
	}
	count := 0
	for _, fr := range frameworks {
		if fr.Match(src) {
			count++
		}
	}
	return count
}

// isFrameworkIntegrationPath reports whether a path lives in a
// framework integration tree where evals belong at the framework's
// downstream-app layer, not the integration itself.
func isFrameworkIntegrationPath(path string) bool {
	lower := strings.ToLower(path)
	patterns := []string{
		"/integrations/llms/",
		"/integrations/llm/",
		"/integrations/providers/",
		"/providers/",
		"/adapters/",
		"/integrations/storage/",
		"/integrations/vector_stores/",
		"/integrations/embeddings/",
		"/connectors/",
		"/integrations/",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// /llms/<provider>/<file>.py style (llama_index, langchain)
	if llmsSubdirAnyPathRE.MatchString(path) {
		return true
	}
	return false
}

var llmsSubdirAnyPathRE = regexp.MustCompile(`(?i)/llms?/[^/]+/`)

// defaultImportWeight matches the calibration table's per-SDK
// import weight when the stage runs without a composer override.
func defaultImportWeight(name string) float64 {
	switch name {
	case "openai", "anthropic":
		return +0.4
	case "langchain", "llama_index":
		return +0.3
	default:
		return +0.3
	}
}

func defaultCallWeight(name string) float64 {
	switch name {
	case "openai", "anthropic":
		return +1.4
	case "langchain":
		return +1.2
	case "llama_index":
		return +1.1
	case "google_genai", "openai_compat":
		return +1.2
	case "sklearn_train", "keras_train":
		return +1.5
	case "xgb_lgb_cat_train", "pytorch_train", "transformers_train":
		return +1.7
	default:
		return +1.0
	}
}

// ctxPair is a (name, anchor regex, verb regex) bundle. Both regexes
// must match in the same file for the atom to fire with positive
// weight.
type ctxPair struct {
	name   string
	anchor *regexp.Regexp
	verb   *regexp.Regexp
}

var ctxPairs = []ctxPair{
	{
		// Python: `import openai`, `from openai import X`
		// JS/TS:  `import OpenAI from 'openai'`, `require('openai')`, `from 'openai'`
		// Go:     `_ "github.com/sashabaranov/go-openai"` (matched via repo-broad regex)
		name: "openai",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+openai\b` +
			`|require\(["']openai["']\)` +
			`|from\s+["']openai["']` +
			`|from\s+["']@openai/[^"']+["']` +
			`|"github\.com/sashabaranov/go-openai"`),
		verb: regexp.MustCompile(`(?:\.|\b)chat\.completions\.create\s*\(` +
			`|(?:\.|\b)completions\.create\s*\(` +
			`|(?:\.|\b)embeddings\.create\s*\(` +
			`|(?:\.|\b)responses\.create\s*\(` +
			`|CreateChatCompletion\s*\(`),
	},
	{
		// Python: `import anthropic`, `from anthropic import Anthropic`
		// JS/TS:  `import Anthropic from '@anthropic-ai/sdk'`
		// Go:     `"github.com/anthropics/anthropic-sdk-go"`
		// Java/Kotlin: `import com.anthropic.client.*`
		name: "anthropic",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+anthropic\b` +
			`|@anthropic-ai/sdk` +
			`|import\s+Anthropic\b` +
			`|"github\.com/anthropics/anthropic-sdk-go"` +
			`|import\s+com\.anthropic\.`),
		verb: regexp.MustCompile(`\.messages\.create\s*\(` +
			`|\.messages\.stream\s*\(` +
			`|Messages\.create\s*\(`),
	},
	{
		// Python: `from langchain[_core|_openai|...] import X`
		//         `from langchain.X.Y import Z` (legacy submodule layout)
		// JS/TS:  `import { ... } from '@langchain/X'` or `from "langchain"`
		name: "langchain",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+langchain(?:_\w+|\.\w+(?:\.\w+)*)?(?:\s+import|\b)` +
			`|@langchain/` +
			`|import\s+\{[^}]*\}\s+from\s+["']langchain["']`),
		verb: regexp.MustCompile(`(?:\b\w+)\.(?:invoke|ainvoke|stream|astream|batch|abatch)\s*\(`),
	},
	{
		// Python: `import llama_index` / `from llama_index import X`
		// JS/TS:  `import { ... } from 'llamaindex'`
		name: "llama_index",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+llama_index\b` +
			`|from\s+["']llamaindex["']` +
			`|require\(["']llamaindex["']\)`),
		verb: regexp.MustCompile(`\b\w+\.(?:query|chat|complete|achat|astream)\s*\(`),
	},
	{
		// Multi-provider OpenAI-compatible: Replicate, Cohere, Mistral,
		// Groq, Together, Fireworks, Perplexity. Multi-language coverage
		// via SDK package name.
		name: "openai_compat",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:replicate|cohere|mistralai|groq|together|fireworks)\b` +
			`|from\s+(?:groq|cohere|together|fireworks)\b` +
			`|require\(["'](?:replicate|cohere-ai|@mistralai/mistralai|groq-sdk|together-ai|@fireworks-ai/sdk)["']\)` +
			`|from\s+["'](?:replicate|cohere-ai|@mistralai/mistralai|groq-sdk|together-ai|@fireworks-ai/sdk)["']`),
		verb: regexp.MustCompile(`(?:\breplicate\.run|\.chat\.completions\.create|\b(?:co|client|cohere)\.(?:generate|chat|embed|rerank))\s*\(`),
	},
	{
		// Google Generative AI / Gemini — Python + JS/TS.
		name: "google_genai",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+google\.(?:generativeai|genai)\b` +
			`|@google/generative-ai` +
			`|@google-cloud/vertexai`),
		verb: regexp.MustCompile(`\b\w+\.generate_content\s*\(` +
			`|getGenerativeModel\s*\(` +
			`|\.generateContent\s*\(`),
	},
	{
		// HuggingFace Inference — Python + JS/TS.
		name: "huggingface",
		anchor: regexp.MustCompile(`(?m)^\s*from\s+huggingface_hub\s+import` +
			`|@huggingface/inference`),
		verb: regexp.MustCompile(`\bInferenceClient\s*\(` +
			`|HfInference\s*\(` +
			`|\.text_generation\s*\(`),
	},
	{
		// LangGraph — Python + JS/TS.
		name: "langgraph",
		anchor: regexp.MustCompile(`(?m)^\s*from\s+langgraph\b` +
			`|@langchain/langgraph`),
		verb: regexp.MustCompile(`\bcreate_react_agent\s*\(` +
			`|StateGraph\s*\(` +
			`|\b\w+\.(?:invoke|ainvoke|stream|astream)\s*\(`),
	},
	{
		name:   "sklearn_train",
		anchor: regexp.MustCompile(`(?m)^\s*from\s+sklearn(?:\b|\.)`),
		verb:   regexp.MustCompile(`\b\w+\.fit\s*\(\s*[XxYy](?:_\w+)?\s*,\s*[Yy]`),
	},
	{
		name:   "xgb_lgb_cat_train",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:xgboost|lightgbm|catboost)\b`),
		verb:   regexp.MustCompile(`\b(?:\w+\.fit\s*\(|xgb\.train\s*\(|lgb\.train\s*\(|(?:XGB|LGBM|CatBoost)(?:Classifier|Regressor)\s*\()`),
	},
	{
		name:   "keras_train",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:tensorflow|keras)\b`),
		verb:   regexp.MustCompile(`\b\w+\.fit\s*\([^)]{0,400}epochs\s*=`),
	},
	{
		name:   "pytorch_train",
		anchor: regexp.MustCompile(`(?m)^\s*(?:from|import)\s+(?:torch|pytorch_lightning|lightning|pl)\b`),
		verb:   regexp.MustCompile(`(?:\boptimizer\.zero_grad\s*\(\s*\)|\btrainer\.fit\s*\(|\bTrainer\s*\([^)]{0,200}\)\.fit\s*\()`),
	},
	{
		name:   "transformers_train",
		anchor: regexp.MustCompile(`(?m)^\s*from\s+transformers\b`),
		verb:   regexp.MustCompile(`\b(?:Trainer\s*\(|trainer\.train\s*\()`),
	},
}

var sdkPresentRE = regexp.MustCompile(`(?m)` +
	`(?:` +
	`\bimport\s+(?:openai|anthropic|cohere|mistralai|groq|together|replicate|fireworks|langchain|llama_index|langgraph|crewai|autogen|tensorflow|torch|keras|sklearn|xgboost|lightgbm|catboost|transformers|pytorch_lightning|lightning)\b` +
	`|\bfrom\s+(?:openai|anthropic|cohere|mistralai|groq|together|replicate|fireworks|langchain(?:_\w+)?|llama_index|langgraph|crewai|autogen|pyautogen|tensorflow|tf\.\w+|torch|keras|sklearn|xgboost|lightgbm|catboost|transformers|pytorch_lightning|lightning|google\.(?:generativeai|genai))\b` +
	`|require\(["'](?:openai|@anthropic-ai/sdk|cohere|@google/generative-ai|replicate|@langchain/[^"']+)["']\)` +
	`|from\s+["'](?:openai|@anthropic-ai/sdk|@langchain/[^"']+)["']` +
	`)`)

var looseVerbRE = regexp.MustCompile(
	`(?:\.|\b)chat\.completions\.create\s*\(` +
		`|(?:\.|\b)completions\.create\s*\(` +
		`|(?:\.|\b)embeddings\.create\s*\(` +
		`|(?:\.|\b)responses\.create\s*\(` +
		`|(?:\.|\b)messages\.create\s*\(` +
		`|\.generate_content\s*\(` +
		`|\b(?:llm|chain|chat_model|chatmodel|agent|graph|chat|qa_chain)\.(?:invoke|ainvoke|stream|astream|batch|abatch)\s*\(` +
		`|\b(?:rf|logreg|lr|nb|gbm|xgb|lgbm|cat|clf|reg|estimator|pipe|pipeline|model|trainer|self\.model|self\.clf|self\.estimator|\w*_model|\w*_clf|\w*_reg)\.fit\s*\(\s*[XxYy](?:_\w+)?\s*,\s*[Yy]` +
		`|\btrainer\.fit\s*\(` +
		`|\bTrainer\s*\([^)]{0,400}\)\.train\s*\(` +
		`|optimizer\.zero_grad\s*\(\s*\)`)

// Wrapper-class detection — the file looks like a provider wrapper:
// a class inherits from an LLM-style base AND defines wrapper-canonical
// methods AND has no module-level call.
var wrapperClassRE = regexp.MustCompile(`(?m)^\s*class\s+\w+\s*\(\s*[^)]*(?:LLM|BaseLLM|BaseChatModel|Provider|BaseProvider|Client|Adapter|Wrapper|Backend|Connector|Embeddings|BaseLanguageModel|BaseEstimator|BaseModel)`)
var wrapperMethodsRE = regexp.MustCompile(`(?m)^\s+(?:async\s+)?def\s+(?:chat|complete|generate|invoke|stream|agenerate|astream|achat|acomplete|embed|_call|_generate|_acall|_stream|fit|fit_transform|partial_fit|transform|predict|score)\s*\(`)

func isWrapperFile(src []byte) bool {
	if !wrapperClassRE.Match(src) {
		return false
	}
	if !wrapperMethodsRE.Match(src) {
		return false
	}
	// If a real module-level call exists, this isn't a pure wrapper.
	if hasModuleLevelCall(src, looseVerbRE, 4) {
		return false
	}
	return true
}

// hasModuleLevelCall scans the source for a verb match at indent ≤
// maxIndent (Python module-level approximation by leading whitespace).
// Skips def/async def/class lines and decorator lines.
func hasModuleLevelCall(src []byte, verbRE *regexp.Regexp, maxIndent int) bool {
	start := 0
	for i := 0; i <= len(src); i++ {
		if i < len(src) && src[i] != '\n' {
			continue
		}
		line := src[start:i]
		start = i + 1
		// Count leading whitespace.
		indent := 0
		j := 0
		for ; j < len(line); j++ {
			if line[j] == ' ' {
				indent++
			} else if line[j] == '\t' {
				indent += 4
			} else {
				break
			}
		}
		if indent > maxIndent {
			continue
		}
		stripped := line[j:]
		if len(stripped) == 0 || stripped[0] == '#' {
			continue
		}
		// Skip def / async def / class / decorator headers.
		if startsWith(stripped, "def ") || startsWith(stripped, "async def ") ||
			startsWith(stripped, "class ") || startsWith(stripped, "@") {
			continue
		}
		if verbRE.Match(line) {
			return true
		}
	}
	return false
}

func startsWith(b []byte, prefix string) bool {
	if len(b) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if b[i] != prefix[i] {
			return false
		}
	}
	return true
}
