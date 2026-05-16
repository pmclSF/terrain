// Package aidetect discovers AI/ML evaluation frameworks, prompt libraries,
// and dataset infrastructure in a repository. It auto-derives scenarios from
// code so users never have to write YAML scenario declarations manually.
//
// Supported frameworks:
//   - promptfoo (config-based prompt evaluation)
//   - deepeval (Python eval framework)
//   - ragas (RAG evaluation)
//   - langchain / langsmith (LLM application framework + observability)
//   - openai (OpenAI SDK patterns)
//   - anthropic (Anthropic SDK patterns)
//   - llamaindex (LLM data framework)
//   - huggingface (transformers/datasets)
//   - pytest-eval (pytest files in eval directories)
//   - vitest-eval (vitest files in eval directories)
package aidetect

// Framework identifies a detected AI/ML evaluation framework.
type Framework struct {
	// Name is the canonical framework identifier.
	Name string `json:"name"`

	// Version is the detected version constraint, if available.
	Version string `json:"version,omitempty"`

	// Source describes how the framework was detected.
	Source string `json:"source"` // "config", "dependency", "import", "directory"

	// ConfigFile is the path to the framework's config file, if any.
	ConfigFile string `json:"configFile,omitempty"`

	// Confidence is how confident we are in the detection (0.0-1.0).
	Confidence float64 `json:"confidence"`
}

// FrameworkSignature defines how to detect a specific framework.
type FrameworkSignature struct {
	Name           string
	ConfigFiles    []string // Config file names to look for
	DependencyKeys []string // package.json or pyproject.toml dependency names
	ImportPatterns []string // Import patterns in source code
}

// KnownFrameworks lists all supported AI/ML framework signatures.
var KnownFrameworks = []FrameworkSignature{
	{
		Name:           "promptfoo",
		ConfigFiles:    []string{"promptfooconfig.yaml", "promptfooconfig.yml", "promptfoo.yaml", ".promptfoo.yaml"},
		DependencyKeys: []string{"promptfoo"},
		ImportPatterns: []string{"promptfoo", "from promptfoo"},
	},
	{
		Name:           "deepeval",
		DependencyKeys: []string{"deepeval"},
		ImportPatterns: []string{"from deepeval", "import deepeval"},
	},
	{
		Name:           "ragas",
		DependencyKeys: []string{"ragas"},
		ImportPatterns: []string{"from ragas", "import ragas"},
	},
	{
		Name:           "langchain",
		DependencyKeys: []string{"langchain", "@langchain/core", "@langchain/openai", "@langchain/anthropic", "langchain-core", "langchain-openai", "langchain-anthropic"},
		ImportPatterns: []string{"from langchain", "import langchain", "@langchain/", "langchain/"},
	},
	{
		Name:           "langsmith",
		DependencyKeys: []string{"langsmith", "@langchain/smith"},
		ImportPatterns: []string{"from langsmith", "import langsmith", "LANGSMITH_API_KEY", "@langchain/smith"},
		ConfigFiles:    []string{".langsmith.yaml", "langsmith.config.ts", "langsmith.config.js"},
	},
	{
		Name:           "openai",
		DependencyKeys: []string{"openai", "@openai/api"},
		ImportPatterns: []string{"from openai", "import openai", "import OpenAI", "from openai import"},
	},
	{
		Name:           "anthropic",
		DependencyKeys: []string{"@anthropic-ai/sdk", "anthropic"},
		ImportPatterns: []string{"from anthropic", "import anthropic", "import Anthropic", "@anthropic-ai/sdk"},
	},
	{
		Name:           "llamaindex",
		DependencyKeys: []string{"llamaindex", "llama-index", "llama_index"},
		ImportPatterns: []string{"from llama_index", "from llamaindex", "import llamaindex"},
	},
	{
		// HuggingFace umbrella: transformers + datasets + hub clients.
		// Matches both LLM and non-LLM uses (BERT-style classifiers,
		// vision models, embeddings). Use huggingface-llm below for the
		// generative subset.
		Name:           "huggingface",
		DependencyKeys: []string{"transformers", "datasets", "@huggingface/inference", "huggingface_hub"},
		ImportPatterns: []string{"from transformers", "from datasets", "import transformers", "@huggingface/", "from huggingface_hub"},
	},
	{
		// HuggingFace LLM subset — fires only when the file uses one of
		// the generative-specific entry points. Coexists with the
		// broader "huggingface" entry; consumers wanting "is this repo
		// generative-LLM" check for this name. Consumers wanting "any
		// ML / NLP transformers" check for "huggingface".
		Name:           "huggingface-llm",
		DependencyKeys: nil,
		ImportPatterns: []string{
			"AutoModelForCausalLM",
			"AutoModelForSeq2SeqLM",
			"LlamaForCausalLM",
			"MistralForCausalLM",
			"GPTNeoXForCausalLM",
			"pipeline(\"text-generation\")",
			"pipeline('text-generation')",
			"pipeline(\"conversational\")",
			"pipeline('conversational')",
			"pipeline(\"text2text-generation\")",
			"pipeline('text2text-generation')",
			"AutoModelForVisualQuestionAnswering",
			"AutoModelForImageTextRetrieval",
		},
	},
	{
		Name:           "vertexai",
		DependencyKeys: []string{"@google-cloud/vertexai", "google-cloud-aiplatform"},
		ImportPatterns: []string{"from google.cloud import aiplatform", "from vertexai", "@google-cloud/vertexai"},
	},
	{
		Name:           "aws-bedrock",
		DependencyKeys: []string{"@aws-sdk/client-bedrock-runtime", "boto3"},
		ImportPatterns: []string{"bedrock-runtime", "from boto3", "invoke_model"},
	},
	{
		Name:           "ollama",
		DependencyKeys: []string{"ollama", "ollama-ai-provider"},
		ImportPatterns: []string{"from ollama", "import ollama", "ollama/browser"},
	},
	{
		Name:           "cohere",
		DependencyKeys: []string{"cohere", "cohere-ai"},
		ImportPatterns: []string{"from cohere", "import cohere", "cohere-ai"},
	},
	{
		Name:           "groq",
		DependencyKeys: []string{"groq", "groq-sdk"},
		ImportPatterns: []string{"from groq", "import groq", "groq-sdk"},
	},
	{
		Name:           "mistral",
		DependencyKeys: []string{"@mistralai/mistralai", "mistralai"},
		ImportPatterns: []string{"from mistralai", "import mistralai", "@mistralai/"},
	},
	{
		Name:           "wandb",
		DependencyKeys: []string{"wandb"},
		ImportPatterns: []string{"import wandb", "from wandb", "WANDB_API_KEY"},
	},
	{
		Name:           "mlflow",
		DependencyKeys: []string{"mlflow"},
		ImportPatterns: []string{"import mlflow", "from mlflow", "mlflow.start_run"},
	},
	{
		Name:           "instructor",
		DependencyKeys: []string{"instructor"},
		ImportPatterns: []string{"import instructor", "from instructor", "instructor.patch"},
	},
	{
		Name:           "guardrails",
		DependencyKeys: []string{"guardrails-ai", "nemoguardrails"},
		ImportPatterns: []string{"from guardrails", "import guardrails", "from nemoguardrails", "import nemoguardrails"},
	},

	// --- Classical ML frameworks (non-LLM) ---
	// These detect repos that ship machine-learning models — typically
	// sklearn / xgboost / lightgbm-trained classifiers or regressors,
	// or deep-learning models built on PyTorch / TensorFlow / JAX.
	// Their detection feeds the lifecycle/* and regression/performance-*
	// rules just as the LLM-frameworks above feed regression/eval-*.

	{
		Name:           "sklearn",
		DependencyKeys: []string{"scikit-learn", "sklearn"},
		ImportPatterns: []string{"from sklearn", "import sklearn", "from sklearn.", "sklearn.metrics"},
	},
	{
		Name:           "xgboost",
		DependencyKeys: []string{"xgboost"},
		ImportPatterns: []string{"import xgboost", "from xgboost", "xgb.XGBClassifier", "xgb.XGBRegressor"},
	},
	{
		Name:           "lightgbm",
		DependencyKeys: []string{"lightgbm"},
		ImportPatterns: []string{"import lightgbm", "from lightgbm", "lgb.LGBMClassifier", "lgb.LGBMRegressor"},
	},
	{
		Name:           "statsmodels",
		DependencyKeys: []string{"statsmodels"},
		ImportPatterns: []string{"import statsmodels", "from statsmodels", "sm.OLS", "sm.Logit"},
	},
	{
		Name:           "pytorch",
		DependencyKeys: []string{"torch", "pytorch-lightning", "pytorch_lightning"},
		ImportPatterns: []string{"import torch", "from torch", "torch.nn", "import pytorch_lightning"},
	},
	{
		Name:           "tensorflow",
		DependencyKeys: []string{"tensorflow", "tf-nightly", "tensorflow-gpu"},
		ImportPatterns: []string{"import tensorflow", "from tensorflow", "import tf", "tf.keras"},
	},
	{
		Name:           "jax",
		DependencyKeys: []string{"jax", "jaxlib", "flax"},
		ImportPatterns: []string{"import jax", "from jax", "jax.numpy", "import flax"},
	},
	{
		Name:           "sagemaker",
		DependencyKeys: []string{"sagemaker", "boto3"},
		ImportPatterns: []string{"import sagemaker", "from sagemaker", "sagemaker.estimator", "sagemaker.predictor"},
	},

	// --- Data validation (broad use, not just LLM outputs) ---
	// pydantic appears in roughly every modern Python repo that does
	// structured I/O — FastAPI request models, settings, eval input
	// contracts, output schemas, you name it. Detection is broad on
	// purpose: presence of pydantic flags a repo as having declared
	// data contracts at all, which lifecycle / hygiene rules use to
	// reason about contract changes. The PydanticOutputParser-only
	// detection in ai_schema_parser.go remains valid as a narrower
	// LLM-specific signal.
	{
		Name:           "pydantic",
		DependencyKeys: []string{"pydantic", "pydantic-settings", "pydantic-core"},
		ImportPatterns: []string{"from pydantic", "import pydantic", "pydantic.BaseModel", "pydantic.Field"},
	},

	// Native Go LLM SDK (community fork of the official Python SDK).
	{
		Name:           "openai-go",
		DependencyKeys: []string{"github.com/sashabaranov/go-openai", "github.com/openai/openai-go"},
		ImportPatterns: []string{"sashabaranov/go-openai", "openai/openai-go", "openai.NewClient"},
	},

	// Java OpenAI clients (official + community).
	{
		Name:           "openai-java",
		DependencyKeys: []string{"com.theokanning.openai-gpt3-java", "com.openai:openai-java"},
		ImportPatterns: []string{"com.theokanning.openai", "import com.openai", "OpenAiService"},
	},

	// --- Pipeline orchestration ---
	// Workflow / DAG frameworks. Detection surfaces them as
	// "pipeline files" — the dbt manifest equivalent for Python
	// orchestration. Rules like lifecycle/orphaned-pipeline and
	// hygiene/no-tasks fire on these. See internal/pipelinedag/ for
	// per-file DAG / flow parsing.
	{
		Name:           "airflow",
		DependencyKeys: []string{"apache-airflow", "airflow"},
		ImportPatterns: []string{"from airflow", "import airflow", "@dag", "from airflow.decorators"},
	},
	{
		Name:           "prefect",
		DependencyKeys: []string{"prefect"},
		ImportPatterns: []string{"from prefect", "import prefect", "@flow", "@task"},
	},
}
