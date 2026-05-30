package slash

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Grammar / Parse ─────────────────────────────────────────────────

func TestParse_NonSlashReturnsNil(t *testing.T) {
	c, err := Parse("not a slash command")
	if c != nil || err != nil {
		t.Errorf("non-slash should return (nil, nil); got (%+v, %v)", c, err)
	}
	c, err = Parse("")
	if c != nil || err != nil {
		t.Errorf("empty should return (nil, nil)")
	}
}

func TestParse_DismissWithReason(t *testing.T) {
	c, err := Parse(`/dismiss reason:"false positive"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Verb != VerbDismiss {
		t.Errorf("verb = %q, want %q", c.Verb, VerbDismiss)
	}
	if c.Keyword["reason"] != "false positive" {
		t.Errorf("reason = %q", c.Keyword["reason"])
	}
}

func TestParse_DismissRequiresReason(t *testing.T) {
	_, err := Parse(`/dismiss`)
	if err == nil {
		t.Fatal("dismiss without reason should error")
	}
	if !strings.Contains(err.Error(), "reason") {
		t.Errorf("error should mention 'reason'; got %v", err)
	}
}

func TestParse_TerrainPrefixOptional(t *testing.T) {
	c1, err := Parse(`/terrain explain ai.surface.missing_eval`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// `/explain` directly should also work — verb-only shape.
	c2, err := Parse(`/explain ai.surface.missing_eval`)
	if err != nil {
		t.Fatalf("parse no-prefix: %v", err)
	}
	if c1.Verb != c2.Verb || c1.Positional[0] != c2.Positional[0] {
		t.Errorf("prefix vs no-prefix differ: %+v vs %+v", c1, c2)
	}
}

func TestParse_UnknownVerbSuggestion(t *testing.T) {
	_, err := Parse(`/terrain explan some-rule`)
	if err == nil {
		t.Fatal("typo should error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Suggestion != "explain" {
		t.Errorf("expected suggestion 'explain', got %q", pe.Suggestion)
	}
}

func TestParse_CaseInsensitiveVerb(t *testing.T) {
	c, err := Parse(`/TERRAIN Dismiss reason:test`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Verb != VerbDismiss {
		t.Errorf("verb = %q, want %q", c.Verb, VerbDismiss)
	}
}

func TestParse_ScaffoldAcceptRequired(t *testing.T) {
	if _, err := Parse(`/terrain scaffold`); err == nil {
		t.Error("scaffold without subverb should error")
	}
	c, err := Parse(`/terrain scaffold accept`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Verb != VerbScaffold {
		t.Errorf("verb = %q", c.Verb)
	}
}

func TestParse_BenchRequiresID(t *testing.T) {
	if _, err := Parse(`/terrain bench`); err == nil {
		t.Error("bench without id should error")
	}
	if _, err := Parse(`/terrain bench latency-001`); err != nil {
		t.Errorf("bench with id should parse: %v", err)
	}
}

func TestParse_ShowRequiresID(t *testing.T) {
	if _, err := Parse(`/terrain show`); err == nil {
		t.Error("show without id should error")
	}
	c, err := Parse(`/terrain show weakAssertion@a.go:X#abc12345`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Positional[0] != "weakAssertion@a.go:X#abc12345" {
		t.Errorf("positional = %v", c.Positional)
	}
}

func TestParse_QuotedReasonWithEscapes(t *testing.T) {
	c, err := Parse(`/dismiss reason:"says \"hello\" politely"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(c.Keyword["reason"], `"hello"`) {
		t.Errorf("escape did not preserve embedded quotes; got %q", c.Keyword["reason"])
	}
}

func TestParse_UnterminatedQuote(t *testing.T) {
	if _, err := Parse(`/dismiss reason:"never closed`); err == nil {
		t.Error("unterminated quote should error")
	}
}

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"explain", "explain", 0},
		{"explan", "explain", 1}, // missing letter
		{"explayn", "explain", 1},
		{"", "abc", 3},
		{"abc", "", 3},
	}
	for _, c := range cases {
		if got := editDistance(c.a, c.b); got != c.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// ── Command list rendering ─────────────────────────────────────────

func TestRenderCommandList_HasAllTenVerbs(t *testing.T) {
	out := RenderCommandList()
	// Each of the 10 verbs should appear in the rendered list.
	for _, verb := range AllVerbs {
		// `/dismiss` is special-cased without `/terrain` prefix in
		// the command-list output; check for either shape.
		v := "/" + string(verb)
		v2 := "/terrain " + string(verb)
		if !strings.Contains(out, v) && !strings.Contains(out, v2) {
			t.Errorf("rendered command list missing verb %q; output:\n%s", verb, out)
		}
	}
}

func TestRenderCommandList_DeterministicAcrossCalls(t *testing.T) {
	a := RenderCommandList()
	b := RenderCommandList()
	if a != b {
		t.Error("command list should be byte-identical across calls")
	}
}

// ── Signature ──────────────────────────────────────────────────────

func TestVerifySignature_Valid(t *testing.T) {
	body := []byte(`{"action":"created"}`)
	secret := "very-secret"
	header := ComputeSignatureHeader(body, secret)
	if err := VerifySignature(header, body, secret); err != nil {
		t.Errorf("expected valid signature, got %v", err)
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	body := []byte(`{"action":"created"}`)
	secret := "very-secret"
	header := ComputeSignatureHeader(body, secret)
	// Mutate the body.
	if err := VerifySignature(header, []byte(`{"action":"different"}`), secret); err == nil {
		t.Error("tampered body should fail signature check")
	}
}

func TestVerifySignature_NoSecret(t *testing.T) {
	if err := VerifySignature("sha256=abc", []byte("x"), ""); err == nil {
		t.Error("empty secret should error")
	}
}

func TestVerifySignature_BadHeader(t *testing.T) {
	body := []byte("x")
	secret := "s"
	if err := VerifySignature("notsha256=abc", body, secret); err == nil {
		t.Error("missing prefix should error")
	}
	if err := VerifySignature("sha256=not-hex", body, secret); err == nil {
		t.Error("non-hex header should error")
	}
}

// ── Webhook handler ─────────────────────────────────────────────────

// echoDispatcher returns the command's verb in the reply for assertion.
type echoDispatcher struct{}

func (echoDispatcher) Handle(_ WebhookEvent, cmd *Command) (string, error) {
	return string(cmd.Verb), nil
}

func makeEvent(t *testing.T, body string, commentBody string) []byte {
	t.Helper()
	payload := map[string]any{
		"action": "created",
		"comment": map[string]any{
			"id":   42,
			"body": commentBody,
			"user": map[string]any{"login": "alice"},
		},
		"issue": map[string]any{"number": 1},
		"repository": map[string]any{
			"full_name": "org/repo",
		},
		"sender": map[string]any{"login": "alice"},
	}
	data, _ := json.Marshal(payload)
	if body != "" {
		return []byte(body)
	}
	return data
}

func TestHandler_NewHandlerRefusesEmptySecret(t *testing.T) {
	if h := NewHandler("", echoDispatcher{}); h != nil {
		t.Error("empty secret should produce nil handler")
	}
}

func TestHandler_RejectsBadSignature(t *testing.T) {
	body := makeEvent(t, "", "/dismiss reason:test")
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=wrong")
	req.Header.Set("X-GitHub-Event", "issue_comment")
	rr := httptest.NewRecorder()
	h := NewHandler("the-secret", echoDispatcher{})
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestHandler_DispatchesDismiss(t *testing.T) {
	body := makeEvent(t, "", `/dismiss reason:"false-positive"`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", ComputeSignatureHeader(body, "the-secret"))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	rr := httptest.NewRecorder()
	NewHandler("the-secret", echoDispatcher{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	out, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(out), "dismiss") {
		t.Errorf("expected reply to contain 'dismiss'; got %s", string(out))
	}
}

func TestHandler_IgnoresBotComments(t *testing.T) {
	payload := map[string]any{
		"action": "created",
		"comment": map[string]any{
			"id":   42,
			"body": "/dismiss reason:x",
			"user": map[string]any{"login": "terrain-bot[bot]"},
		},
		"issue":      map[string]any{"number": 1},
		"repository": map[string]any{"full_name": "org/repo"},
		"sender":     map[string]any{"login": "terrain-bot[bot]"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", ComputeSignatureHeader(body, "secret"))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	rr := httptest.NewRecorder()
	NewHandler("secret", echoDispatcher{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status: %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ignored (bot sender)") {
		t.Errorf("expected bot-ignore message; got %s", rr.Body.String())
	}
}

func TestHandler_AcceptsPing(t *testing.T) {
	body := []byte(`{"zen":"pong"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", ComputeSignatureHeader(body, "secret"))
	req.Header.Set("X-GitHub-Event", "ping")
	rr := httptest.NewRecorder()
	NewHandler("secret", echoDispatcher{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestHandler_RejectsNonPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rr := httptest.NewRecorder()
	NewHandler("secret", echoDispatcher{}).ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: %d", rr.Code)
	}
}

func TestHandler_MultiCommandReply(t *testing.T) {
	body := makeEvent(t, "", "/terrain show abc\n/terrain refresh")
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", ComputeSignatureHeader(body, "secret"))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	rr := httptest.NewRecorder()
	NewHandler("secret", echoDispatcher{}).ServeHTTP(rr, req)
	out := rr.Body.String()
	if !strings.Contains(out, "show") || !strings.Contains(out, "refresh") {
		t.Errorf("multi-command reply missing verbs; got:\n%s", out)
	}
}

// ── Default dispatcher ─────────────────────────────────────────────

func TestDefaultDispatcher_CommandsRendersList(t *testing.T) {
	cmd, _ := Parse(`/terrain commands`)
	r, err := (DefaultDispatcher{}).Handle(WebhookEvent{}, cmd)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(r, "Terrain — slash commands") {
		t.Errorf("expected command-list header in reply; got:\n%s", r)
	}
}

// /terrain scaffold accept should give a useful, actionable reply.
// Without keyword args, it documents the usage. With schema:<path>,
// it surfaces the equivalent CLI invocation an adopter can run.
func TestDefaultDispatcher_ScaffoldWithoutSchemaShowsUsage(t *testing.T) {
	cmd, err := Parse(`/terrain scaffold accept`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r, err := (DefaultDispatcher{}).Handle(WebhookEvent{}, cmd)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(r, "schema:<path>") {
		t.Errorf("expected usage hint to mention schema:<path>; got:\n%s", r)
	}
	if !strings.Contains(r, "terrain scaffold --schema") {
		t.Errorf("expected reply to surface CLI command; got:\n%s", r)
	}
}

func TestDefaultDispatcher_ScaffoldWithSchemaShowsCLICommand(t *testing.T) {
	cmd, err := Parse(`/terrain scaffold accept schema:schemas/input.json prompt:prompts/main.md`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r, err := (DefaultDispatcher{}).Handle(WebhookEvent{}, cmd)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(r, "schemas/input.json") {
		t.Errorf("expected schema path echoed in reply; got:\n%s", r)
	}
	if !strings.Contains(r, "prompts/main.md") {
		t.Errorf("expected prompt path echoed in reply; got:\n%s", r)
	}
	if !strings.Contains(r, "terrain scaffold --schema schemas/input.json --prompt prompts/main.md --lang python") {
		t.Errorf("expected full CLI invocation in reply; got:\n%s", r)
	}
}
