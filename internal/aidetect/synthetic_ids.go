package aidetect

import (
	"regexp"
	"strings"
)

// Synthetic identifier helpers. AI surface detectors flag named
// "surfaces" (model ids, prompt names, agent ids) for downstream gates
// such as surfacelit (which requires the named surface to appear as a
// literal token in source). Surface extractors sometimes synthesize
// names for constructor-derived or unnamed surfaces — these labels do
// NOT appear in source, so the literal-presence gate would wrongly
// suppress every finding on them. The helpers here recognize the
// synthetic naming convention so consumer detectors can skip the gate
// for those identifiers.

// syntheticLineSuffixRe matches the "_L<line>" suffix the surface
// extractor synthesizes for unnamed surfaces.
var syntheticLineSuffixRe = regexp.MustCompile(`_L\d+$`)

// syntheticPrefixes are the known synthetic-label prefixes the
// surface extractor emits. Surfaces named with any of these prefixes
// are constructor / framework / array-shape derived; the label itself
// doesn't appear as a literal token in source so a presence check
// would always fail.
var syntheticPrefixes = []string{
	"sdk_client_", "llm_call_", "framework_msg_",
	"template_prompt_", "api_prompt_", "system_prompt_",
	"message_array_", "message_list_",
	"few_shot_", "prompt_const_", "dspy_",
}

// syntheticExactMatches are constructor-derived labels that don't
// follow a "<prefix>_<var>" shape.
var syntheticExactMatches = map[string]bool{
	"structured_output":   true,
	"message_slice":       true,
	"message_array":       true,
	"template_file":       true,
	"system_message":      true,
	"user_message":        true,
	"assistant_message":   true,
	"vector_store":        true,
	"vector_store_chroma": true,
	"vector_store_faiss":  true,
	"vector_store_config": true,
	"embedding_model":     true,
	"retriever_config":    true,
	"prompt_template":     true,
	"system_prompt":       true,
	"user_prompt":         true,
	"rag_pipeline":        true,
	"langchain_message":   true,
	"llamaindex_message":  true,
	"chunking_config":     true,
	"reranker_config":     true,
	"retrieval_query":     true,
	"rag_component":       true,
}

// IsSyntheticIdentifier reports whether `id` is a constructor-driven
// or otherwise synthetic identifier label that will not appear as a
// literal token in source. Consumer detectors call this before
// applying the surfacelit gate so a synthetic label is not wrongly
// suppressed.
func IsSyntheticIdentifier(id string) bool {
	if id == "" || strings.ContainsAny(id, "( ") {
		return true
	}
	if syntheticLineSuffixRe.MatchString(id) {
		return true
	}
	if syntheticExactMatches[id] {
		return true
	}
	for _, p := range syntheticPrefixes {
		if strings.HasPrefix(id, p) {
			return true
		}
	}
	return false
}
