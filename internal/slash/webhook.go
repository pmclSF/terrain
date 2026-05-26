package slash

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// WebhookEvent is the parsed shape of a GitHub `issue_comment` or
// `pull_request_review_comment` event after the slash-command body
// has been extracted. Only the fields the dispatcher needs.
type WebhookEvent struct {
	// Action is the GitHub event action (e.g. "created", "edited").
	// Terrain only acts on "created" by default.
	Action string `json:"action"`
	// Sender is the GitHub login of the comment author. Used to
	// filter bot-authored comments so Terrain doesn't act on its
	// own posts.
	Sender string `json:"sender"`
	// CommentID is the GitHub comment ID. Used for replies.
	CommentID int64 `json:"comment_id"`
	// CommentBody is the raw comment markdown.
	CommentBody string `json:"comment_body"`
	// PRNumber is the pull-request number the comment is on.
	PRNumber int `json:"pr_number"`
	// Repository is "owner/name".
	Repository string `json:"repository"`
	// FindingID is the finding the comment is attached to, when the
	// comment is a reply to a Terrain-posted inline annotation.
	// Empty for top-level PR comments.
	FindingID string `json:"finding_id,omitempty"`
}

// Dispatcher executes parsed Commands against the snapshot/repo.
// Implementations are typically backed by the existing CLI runners
// (runShow, runExplain, runSuppress, etc.) — slash commands are a
// remote-control surface over the same primitives.
//
// Each Handle returns a markdown response that the webhook handler
// posts back to the PR as a reply comment.
type Dispatcher interface {
	// Handle dispatches the command and returns the reply markdown.
	// An error is treated as a 500 by the HTTP handler; the message
	// is written to the response body (the GitHub event delivery
	// retry path expects 5xx for transient failures).
	Handle(ev WebhookEvent, cmd *Command) (replyMarkdown string, err error)
}

// Handler is the HTTP handler for GitHub webhook deliveries.
// Construct with NewHandler(secret, dispatcher). Wire to a server
// (`internal/server`, terrain serve --webhook) or any net/http
// server.
type Handler struct {
	Secret     string
	Dispatcher Dispatcher
}

// NewHandler builds a webhook handler. Returns nil when secret is
// empty — the package refuses to accept unsigned webhooks.
func NewHandler(secret string, dispatcher Dispatcher) *Handler {
	if secret == "" {
		return nil
	}
	return &Handler{Secret: secret, Dispatcher: dispatcher}
}

// ServeHTTP implements http.Handler. Validates the signature, parses
// the event, extracts any slash commands from the comment body, and
// dispatches each through h.Dispatcher.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := VerifySignature(r.Header.Get("X-Hub-Signature-256"), body, h.Secret); err != nil {
		http.Error(w, "signature: "+err.Error(), http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	switch eventType {
	case "ping":
		// GitHub sends a `ping` event when a webhook is first
		// configured. Return 200 with the standard reply so the
		// hook shows as "delivered" in the UI.
		_, _ = w.Write([]byte("pong"))
		return
	case "issue_comment", "pull_request_review_comment":
		// Continue to slash-command parsing.
	default:
		// Quietly accept other event types so unsubscribing is a
		// GitHub-side configuration rather than a 4xx noise spike.
		_, _ = w.Write([]byte("ignored"))
		return
	}

	ev, err := parseEvent(eventType, body)
	if err != nil {
		http.Error(w, "parse event: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Skip Terrain's own comments to avoid feedback loops.
	if strings.HasSuffix(strings.ToLower(ev.Sender), "[bot]") {
		_, _ = w.Write([]byte("ignored (bot sender)"))
		return
	}

	// Extract slash commands from each line of the comment body.
	// Multi-command comments are supported; each runs independently.
	cmds, parseErrs := parseAllLines(ev.CommentBody)
	if len(cmds) == 0 && len(parseErrs) == 0 {
		// Not a slash command — nothing to do.
		_, _ = w.Write([]byte("no slash commands"))
		return
	}

	var replyLines []string
	for _, pe := range parseErrs {
		replyLines = append(replyLines, "**Parse error**: "+pe.Error())
	}
	for _, cmd := range cmds {
		reply, derr := h.Dispatcher.Handle(ev, cmd)
		if derr != nil {
			replyLines = append(replyLines, fmt.Sprintf("**Error running `%s`**: %v", cmd.Verb, derr))
			continue
		}
		if reply != "" {
			replyLines = append(replyLines, reply)
		}
	}

	body = []byte(strings.Join(replyLines, "\n\n---\n\n"))
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write(body)
}

// parseAllLines walks the comment body line-by-line, extracting
// slash commands. Returns the successfully-parsed commands plus a
// slice of parse errors for malformed lines.
func parseAllLines(body string) ([]*Command, []*ParseError) {
	var cmds []*Command
	var errs []*ParseError
	for _, line := range strings.Split(body, "\n") {
		cmd, err := Parse(line)
		if err != nil {
			if pe, ok := err.(*ParseError); ok {
				errs = append(errs, pe)
			}
			continue
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds, errs
}

// parseEvent extracts the WebhookEvent shape from a GitHub event
// payload. The two relevant event types
// (`issue_comment`, `pull_request_review_comment`) have slightly
// different JSON shapes; we map both to the unified WebhookEvent.
func parseEvent(eventType string, body []byte) (WebhookEvent, error) {
	// Generic shape covering both event types.
	var raw struct {
		Action  string `json:"action"`
		Comment struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
			Path string `json:"path,omitempty"` // pull_request_review_comment only
		} `json:"comment"`
		Issue struct {
			Number int `json:"number"`
		} `json:"issue,omitempty"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request,omitempty"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return WebhookEvent{}, err
	}
	ev := WebhookEvent{
		Action:      raw.Action,
		Sender:      raw.Sender.Login,
		CommentID:   raw.Comment.ID,
		CommentBody: raw.Comment.Body,
		Repository:  raw.Repository.FullName,
	}
	switch eventType {
	case "issue_comment":
		ev.PRNumber = raw.Issue.Number
	case "pull_request_review_comment":
		ev.PRNumber = raw.PullRequest.Number
	}
	return ev, nil
}
