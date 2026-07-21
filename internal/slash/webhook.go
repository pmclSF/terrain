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

// DismissPolicy controls who may invoke /dismiss via the webhook.
// Default (zero value) is deny-all: webhook /dismiss replies with a
// "not authorized" notice rather than writing to .terrain/suppressions.yaml.
// The handler still echoes the parsed command back so the user knows
// it was received.
//
// The deny-by-default posture exists because the webhook receives an
// HMAC-signed payload that establishes "this came from GitHub" but
// not "this user is authorized to suppress a finding." Without an
// explicit policy, Terrain refuses to act on the implicit assumption.
//
// Adopters opt in by configuring terrain.yaml:
//
//	slash:
//	  dismiss:
//	    allow_authors: ["alice", "bob"]   # explicit allowlist
//	    # or:
//	    allow_anyone_with_comment_access: true   # accept any commenter
type DismissPolicy struct {
	// AllowAuthors is the explicit allowlist of GitHub logins permitted
	// to invoke /dismiss. Empty by default.
	AllowAuthors []string

	// AllowAnyoneWithCommentAccess removes the allowlist gate. Set
	// this only when adopters trust their PR comment access to imply
	// dismiss authority (typical for a small private team).
	AllowAnyoneWithCommentAccess bool
}

// AllowsDismiss reports whether sender (a GitHub login) is permitted
// to invoke /dismiss under this policy.
func (p DismissPolicy) AllowsDismiss(sender string) bool {
	if p.AllowAnyoneWithCommentAccess {
		return true
	}
	for _, a := range p.AllowAuthors {
		if a == sender {
			return true
		}
	}
	return false
}

// Handler is the HTTP handler for GitHub webhook deliveries.
// Construct with NewHandler(secret, dispatcher). Wire to a server
// (`internal/server`, terrain serve --webhook) or any net/http
// server.
type Handler struct {
	Secret        string
	Dispatcher    Dispatcher
	DismissPolicy DismissPolicy
}

// NewHandler builds a webhook handler. Returns nil when secret is
// empty — the package refuses to accept unsigned webhooks.
//
// The returned Handler has a zero DismissPolicy (deny-all). Adopters
// who want /dismiss to do anything in production must set
// h.DismissPolicy explicitly. See DismissPolicy for the contract.
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
	// Terrain itself does no outbound HTTP, so it cannot fetch the
	// parent comment to read its finding-id marker. The proxy layer
	// (examples/slash-proxy/) is responsible for resolving the parent
	// and forwarding the id in this header. As a fallback the user
	// can include `finding:<id>` directly in their slash command (see
	// dispatcher handling).
	if hdr := r.Header.Get("X-Terrain-Finding-Id"); hdr != "" {
		ev.FindingID = hdr
	}

	// Only act on newly-created comments. An `edited` or `deleted`
	// action would otherwise re-fire the slash commands in the body
	// (e.g. re-running /dismiss and re-writing the suppression) every
	// time the comment is touched. An empty action is accepted so
	// hand-built / proxied payloads that omit it still work.
	if ev.Action != "" && ev.Action != "created" {
		_, _ = w.Write([]byte("ignored (non-created action)"))
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
		// Authorize destructive commands before dispatch. /dismiss
		// writes to the repo's suppressions file; without a policy
		// check, any PR commenter would be able to suppress arbitrary
		// findings.
		if cmd.Verb == VerbDismiss && !h.DismissPolicy.AllowsDismiss(ev.Sender) {
			replyLines = append(replyLines,
				fmt.Sprintf("`/dismiss` not authorized for `@%s` under this repo's slash policy. "+
					"A maintainer must add the login to `slash.dismiss.allow_authors` in "+
					"`terrain.yaml`, or set `allow_anyone_with_comment_access: true`.", ev.Sender))
			continue
		}
		reply, derr := h.Dispatcher.Handle(ev, cmd)
		if derr != nil {
			replyLines = append(replyLines, fmt.Sprintf("**Error running `%s`**: %v", cmd.Verb, derr))
			continue
		}
		if reply != "" {
			replyLines = append(replyLines, reply)
		}
	}

	// Every command produced an empty reply (and there were no parse
	// errors) — write an explicit body rather than a bare 200 so the
	// delivery is legible in GitHub's webhook UI.
	if len(replyLines) == 0 {
		_, _ = w.Write([]byte("no reply generated"))
		return
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
