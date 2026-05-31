package aidetect

import "testing"

// TestIsSyntheticIdentifier covers every branch of the denylist to
// pin the synthetic-id contract: any name the surface extractor
// synthesizes (constructor-derived, line-suffix, exact-match curated
// list, free-form prefix) must report true so the surfacelit gate
// does not require a literal-token presence for it.
func TestIsSyntheticIdentifier(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want bool
	}{
		// Empty + structurally-impossible identifiers
		{"empty", "", true},
		{"has space", "foo bar", true},
		{"has open paren", "OpenAI(model='gpt-4'", true},

		// Line-suffix regex
		{"line suffix L1", "structured_output_L1", true},
		{"line suffix L42", "system_prompt_L42", true},
		{"line suffix L100", "anything_L100", true},
		{"trailing L only no digits", "anything_L", false},

		// Prefix denylist — every entry
		{"sdk_client_", "sdk_client_OpenAI", true},
		{"llm_call_", "llm_call_chat", true},
		{"framework_msg_", "framework_msg_x", true},
		{"template_prompt_", "template_prompt_main", true},
		{"api_prompt_", "api_prompt_v1", true},
		{"system_prompt_", "system_prompt_safety", true},
		{"message_array_", "message_array_x", true},
		{"message_list_", "message_list_x", true},
		{"few_shot_", "few_shot_example", true},
		{"prompt_const_", "prompt_const_intro", true},
		{"dspy_", "dspy_signature", true},

		// Exact matches — every entry
		{"structured_output", "structured_output", true},
		{"message_slice", "message_slice", true},
		{"message_array", "message_array", true},
		{"template_file", "template_file", true},
		{"system_message", "system_message", true},
		{"user_message", "user_message", true},
		{"assistant_message", "assistant_message", true},
		{"vector_store", "vector_store", true},
		{"vector_store_chroma", "vector_store_chroma", true},
		{"vector_store_faiss", "vector_store_faiss", true},
		{"vector_store_config", "vector_store_config", true},
		{"embedding_model", "embedding_model", true},
		{"retriever_config", "retriever_config", true},
		{"prompt_template", "prompt_template", true},
		{"system_prompt", "system_prompt", true},
		{"user_prompt", "user_prompt", true},
		{"rag_pipeline", "rag_pipeline", true},
		{"langchain_message", "langchain_message", true},
		{"llamaindex_message", "llamaindex_message", true},
		{"chunking_config", "chunking_config", true},
		{"reranker_config", "reranker_config", true},
		{"retrieval_query", "retrieval_query", true},
		{"rag_component", "rag_component", true},

		// Negatives — real identifiers that should NOT be treated as synthetic
		{"real openai model id", "gpt-4o-mini", false},
		{"real anthropic model id", "claude-sonnet-4-5", false},
		{"real ada embed model", "text-embedding-3-large", false},
		{"normal symbol", "MyAgent", false},
		{"snake_case but unknown", "do_something_useful", false},
		{"prefix-like but no underscore", "promptfoo", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSyntheticIdentifier(tc.id)
			if got != tc.want {
				t.Errorf("IsSyntheticIdentifier(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
