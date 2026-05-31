#!/usr/bin/env bash
# Minimal slash-proxy: receives a GitHub webhook, forwards to the
# terrain receiver, takes the reply text, posts it back to the PR
# thread as a comment.
#
# Deploy alongside the terrain webhook receiver. The terrain receiver
# itself does no outbound HTTP; this script is the glue.
#
# Required env:
#   GITHUB_TOKEN              token with repo:issues:write
#   TERRAIN_RECEIVER_URL      http://localhost:8080/webhook (or wherever)
#   GITHUB_WEBHOOK_SECRET     same secret terrain validates against
#
# Usage:
#   Bind this behind a tiny HTTP server (caddy, traefik, fly proxy)
#   that POSTs the raw webhook body to this script's stdin and passes
#   GitHub's X-Hub-Signature-256 and X-GitHub-Event headers through.

set -euo pipefail

: "${GITHUB_TOKEN:?GITHUB_TOKEN required}"
: "${TERRAIN_RECEIVER_URL:?TERRAIN_RECEIVER_URL required}"

BODY="$(cat)"
SIG="${HTTP_X_HUB_SIGNATURE_256:-}"
EVENT="${HTTP_X_GITHUB_EVENT:-}"

# Resolve the finding-id from the parent comment when the user replied
# to a Terrain inline comment. Terrain renders each finding card with a
# hidden `<!-- terrain:finding=<id> -->` marker; this proxy fetches the
# parent comment via the GitHub API (terrain itself does no outbound
# HTTP) and forwards the id in a header.
REPO="$(echo "$BODY" | jq -r '.repository.full_name // empty')"
IN_REPLY_TO="$(echo "$BODY" | jq -r '.comment.in_reply_to_id // empty')"
FINDING_ID=""
if [ -n "$REPO" ] && [ -n "$IN_REPLY_TO" ]; then
  PARENT="$(curl -sf \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/$REPO/pulls/comments/$IN_REPLY_TO" \
    | jq -r '.body // empty')"
  # Match the full finding-id token. IDs can contain `-` (kebab-case
  # path segments are common in TS/JS/Go repos) and `.` and `/` —
  # the only terminator is whitespace or the closing `-->` of the
  # HTML comment. Trim a trailing `-` left from the `-->` boundary
  # since grep can't lookahead.
  FINDING_ID="$(echo "$PARENT" \
    | grep -oE 'terrain:finding=[A-Za-z0-9_/@:#.+\-]+' \
    | head -1 \
    | sed -e 's/^terrain:finding=//' -e 's/-*$//')"
  # The renderer rewrites "--" → "__" inside the marker because HTML
  # comments forbid "--" in the body. Reverse the substitution before
  # forwarding so the receiver sees the original finding-id and the
  # suppression file gets the right key.
  FINDING_ID="$(echo "$FINDING_ID" | sed 's/__/--/g')"
fi

# Forward to terrain. The receiver validates the signature itself.
REPLY="$(curl -sf -X POST "$TERRAIN_RECEIVER_URL" \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: $SIG" \
  -H "X-GitHub-Event: $EVENT" \
  -H "X-Terrain-Finding-Id: $FINDING_ID" \
  --data "$BODY")"

# Empty reply (e.g. an unrelated webhook) → noop, return 200.
if [ -z "$REPLY" ]; then
  exit 0
fi

# Extract repo + issue from the original event.
REPO="$(echo "$BODY" | jq -r '.repository.full_name')"
ISSUE="$(echo "$BODY" | jq -r '.issue.number // .pull_request.number')"

if [ -z "$REPO" ] || [ "$REPO" = "null" ] || [ -z "$ISSUE" ] || [ "$ISSUE" = "null" ]; then
  echo "skip: not a comment event" >&2
  exit 0
fi

# Post the reply back as a PR comment.
jq -n --arg body "$REPLY" '{body: $body}' \
  | curl -sf -X POST "https://api.github.com/repos/$REPO/issues/$ISSUE/comments" \
      -H "Authorization: Bearer $GITHUB_TOKEN" \
      -H "Accept: application/vnd.github+json" \
      --data @-

echo "ok" >&2
