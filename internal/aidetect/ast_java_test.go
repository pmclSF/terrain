package aidetect

import "testing"

func TestDetectJavaAISurfaces_OpenAITheokanning(t *testing.T) {
	t.Parallel()
	src := []byte(`package com.example;

import com.theokanning.openai.service.OpenAiService;
import com.theokanning.openai.completion.chat.ChatCompletionRequest;
import java.util.List;

public class Summarizer {
    private final OpenAiService service;

    public Summarizer(String apiKey) {
        this.service = new OpenAiService(apiKey);
    }

    public String summarize(String text) {
        ChatCompletionRequest req = ChatCompletionRequest.builder()
            .model("gpt-4o-mini")
            .build();
        return service.createChatCompletion(req).getChoices().get(0).getMessage().getContent();
    }
}
`)
	hits := DetectJavaAISurfaces(src, "Summarizer.java")
	if len(hits) < 2 {
		t.Fatalf("hits = %d, want ≥2 (new OpenAiService + service.createChatCompletion): %+v", len(hits), hits)
	}

	var ctor, chatCall *AICallSite
	gotModel := ""
	for i, h := range hits {
		switch {
		case h.Method == "new OpenAiService":
			ctor = &hits[i]
		case h.Method == "service.createChatCompletion":
			chatCall = &hits[i]
		}
		if h.Model != "" && gotModel == "" {
			gotModel = h.Model
		}
	}
	if ctor == nil {
		t.Errorf("missing OpenAiService ctor hit: %+v", hits)
	} else if ctor.SDK != "openai" {
		t.Errorf("ctor SDK = %q", ctor.SDK)
	}
	if chatCall == nil {
		t.Fatalf("missing createChatCompletion hit: %+v", hits)
	}
	if chatCall.SDK != "openai" {
		t.Errorf("chatCall SDK = %q, want openai", chatCall.SDK)
	}
	// Java's builder pattern sets `.model(...)` on a separate statement
	// from the SDK call. The detector extracts the model from whichever
	// invocation hit happens to contain the `.model(...)` chain. The
	// chatCall hit doesn't carry the model arg, but SOME hit in the
	// file should — assert on the collection rather than a specific row.
	if gotModel != "gpt-4o-mini" {
		t.Errorf("expected some hit with model=gpt-4o-mini, got %q across hits %+v", gotModel, hits)
	}
}

func TestDetectJavaAISurfaces_AnthropicOfficial(t *testing.T) {
	t.Parallel()
	src := []byte(`package com.example;

import com.anthropic.client.AnthropicClient;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

public class App {
    public static void main(String[] args) {
        AnthropicClient client = new AnthropicClient(System.getenv("ANTHROPIC_API_KEY"));
        MessageCreateParams params = MessageCreateParams.builder()
            .model("claude-opus-4-7")
            .maxTokens(1024)
            .build();
        client.messages().create(params);
    }
}
`)
	hits := DetectJavaAISurfaces(src, "App.java")
	if len(hits) == 0 {
		t.Fatalf("no hits: %+v", hits)
	}
	gotAnthropic := false
	for _, h := range hits {
		if h.SDK == "anthropic" {
			gotAnthropic = true
		}
	}
	if !gotAnthropic {
		t.Errorf("expected at least one anthropic hit, got %+v", hits)
	}
}

func TestDetectJavaAISurfaces_NoAIImports(t *testing.T) {
	t.Parallel()
	src := []byte(`package com.example;

import java.util.List;

public class Plain {
    public void createChatCompletion() {} // shape-match candidate

    public static void main(String[] args) {
        Plain p = new Plain();
        p.createChatCompletion(); // should NOT fire
    }
}
`)
	hits := DetectJavaAISurfaces(src, "Plain.java")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %+v", hits)
	}
}

func TestClassifyJavaPackage(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"com.theokanning.openai.service.OpenAiService", "openai"},
		{"com.openai.client.OpenAIClient", "openai"},
		{"com.azure.ai.openai.OpenAIClient", "openai"},
		{"com.anthropic.client.AnthropicClient", "anthropic"},
		{"dev.langchain4j.model.openai.OpenAiChatModel", "langchain"},
		{"java.util.List", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := classifyJavaPackage(tc.in); got != tc.want {
			t.Errorf("classifyJavaPackage(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
