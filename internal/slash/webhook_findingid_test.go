package slash

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureDispatcher records the Handle invocation so the test can
// assert what reached the dispatcher.
type captureDispatcher struct {
	gotEvent   WebhookEvent
	gotCommand *Command
}

func (c *captureDispatcher) Handle(ev WebhookEvent, cmd *Command) (string, error) {
	c.gotEvent = ev
	c.gotCommand = cmd
	return "ok", nil
}

// signRequest returns the X-Hub-Signature-256 value for body+secret.
func signRequest(t *testing.T, body, secret []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestWebhook_XTerrainFindingIdHeaderPopulatesEvent is the acceptance
// test for the proxy-injected finding-id contract. When the
// X-Terrain-Finding-Id header is set, the dispatcher MUST see it on
// WebhookEvent.FindingID. Without this, `/dismiss` from a real
// GitHub thread reply has no way to resolve which finding it targets.
func TestWebhook_XTerrainFindingIdHeaderPopulatesEvent(t *testing.T) {
	secret := []byte("test-secret")
	dispatcher := &captureDispatcher{}
	handler := NewHandler(string(secret), dispatcher)

	body := []byte(`{
		"action": "created",
		"comment": {"id": 123, "body": "/dismiss reason:foo", "user": {"login": "octocat"}},
		"issue": {"number": 42},
		"repository": {"full_name": "acme/widget"},
		"sender": {"login": "octocat"}
	}`)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signRequest(t, body, secret))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-Terrain-Finding-Id", "untestedExport@src/x.ts:foo#abc")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if dispatcher.gotEvent.FindingID != "untestedExport@src/x.ts:foo#abc" {
		t.Errorf("FindingID = %q, want the header value", dispatcher.gotEvent.FindingID)
	}
}

// TestWebhook_NoHeaderLeavesFindingIDEmpty proves the header is the
// *only* source of finding-id in the webhook layer. The dispatcher's
// fallback (parsing `finding:<id>` from the slash command keyword)
// is tested separately in cmd/terrain/cmd_slash_dispatcher_test.go.
func TestWebhook_NoHeaderLeavesFindingIDEmpty(t *testing.T) {
	secret := []byte("test-secret")
	dispatcher := &captureDispatcher{}
	handler := NewHandler(string(secret), dispatcher)

	body := []byte(`{
		"action": "created",
		"comment": {"id": 123, "body": "/dismiss reason:foo", "user": {"login": "octocat"}},
		"issue": {"number": 42},
		"repository": {"full_name": "acme/widget"},
		"sender": {"login": "octocat"}
	}`)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signRequest(t, body, secret))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	// No X-Terrain-Finding-Id header.

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if dispatcher.gotEvent.FindingID != "" {
		t.Errorf("FindingID should be empty when no header is set; got %q", dispatcher.gotEvent.FindingID)
	}
}
