# GitHub Checks + slash-command integration

Terrain emits structured outputs. GitHub auth + transport live in your
workflow — the binary never makes outbound HTTP calls.

## The two check runs

Each PR analysis produces two check-run bodies:

| Check run | Required? | Conclusion |
|---|---|---|
| `terrain (gate)` | yes | `failure` when an undismissed gate-tier finding fires; `success` otherwise |
| `terrain (observability)` | no | always `neutral` (informational footer of findings the gate doesn't block on) |

Generate both from one analyze pass:

```bash
terrain report check-runs \
  --head-sha "$GITHUB_SHA" \
  --out /tmp/terrain-checks.json
```

The bundle has two top-level keys — `gate_check` and
`observability_check` — each shaped exactly like a `POST /check-runs`
request body. Split + post:

```bash
jq '.gate_check'          /tmp/terrain-checks.json > /tmp/check-gate.json
jq '.observability_check' /tmp/terrain-checks.json > /tmp/check-obs.json

gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  "/repos/$GITHUB_REPOSITORY/check-runs" \
  --input /tmp/check-gate.json

gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  "/repos/$GITHUB_REPOSITORY/check-runs" \
  --input /tmp/check-obs.json
```

`.github/workflows/terrain-pr.yml` in this repo is the reference
implementation.

### Required permissions

The workflow needs `checks: write` (plus the existing `contents: read`
and `pull-requests: write` for the PR-comment job). The default
`GITHUB_TOKEN` issued to a workflow run is enough — no PAT, no app
installation, no extra secrets.

```yaml
permissions:
  contents: read
  pull-requests: write
  checks: write
```

### Branch-protection wiring

Mark `terrain (gate)` as a required check in the branch-protection
rules for `main`. Leave `terrain (observability)` unselected — that
check should never block a merge.

## The slash-command receiver

`/dismiss`, `/terrain explain`, `/terrain show`, `/terrain commands`
arrive as GitHub `issue_comment` webhook deliveries. Terrain ships a
receiver binary:

```bash
TERRAIN_DEV=1 \
TERRAIN_WEBHOOK_SECRET="<same secret you set on the GitHub webhook>" \
  terrain webhook --addr=":8080"
```

The receiver:

- validates each request's `X-Hub-Signature-256` against the secret
  (HMAC-SHA256 of the raw body),
- parses the comment body for slash verbs,
- dispatches to the in-process runner — `/dismiss` writes a
  suppression to `.terrain/suppressions.yaml`, `/terrain explain`
  renders rule docs, `/terrain show` renders one finding,
- responds 200 with the reply text in the body.

The receiver is a long-lived service — it does not belong inside a
per-PR GitHub Action workflow. Deploy it next to your repo's git
host (a small VM, a Fly machine, a Cloud Run service). A reference
container is below.

### Reference Dockerfile

```dockerfile
FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/terrain ./cmd/terrain

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/terrain /usr/local/bin/terrain
EXPOSE 8080
ENV TERRAIN_DEV=1
USER nonroot
ENTRYPOINT ["/usr/local/bin/terrain", "webhook", "--addr=:8080"]
```

`TERRAIN_WEBHOOK_SECRET` is set in the deploy environment (Cloud Run
secret, Fly secret, Kubernetes Secret) — never in the image, never
in source.

### GitHub webhook configuration

Repository → Settings → Webhooks → Add webhook:

| Field | Value |
|---|---|
| Payload URL | `https://<your-receiver-host>/webhook` |
| Content type | `application/json` |
| Secret | the value you put in `TERRAIN_WEBHOOK_SECRET` |
| Events | `Issue comments` (only) |

The receiver replies in the HTTP response body. To surface the reply
back on the PR thread, run it behind a thin proxy that re-posts the
body through `gh api repos/:owner/:repo/issues/:n/comments`. The
proxy is not part of terrain — it's deployment glue. A minimal
example lives under `examples/slash-proxy/`.

### Finding-id resolution

Every Terrain PR-comment finding card embeds a hidden HTML marker
the proxy reads to resolve `/dismiss` replies:

```
- **`src/auth/login.ts`** [GATE] — Untested export...
  → Add a unit test covering the success and 401 branches.
  <!-- terrain:finding=untestedExport@src/auth/login.ts:loginUser#abc -->
```

The proxy walks the comment thread's `in_reply_to_id` to the parent
comment via `GET /repos/:owner/:repo/pulls/comments/:id`, greps the
parent body for `terrain:finding=`, and forwards the id to the
receiver via the `X-Terrain-Finding-Id` header. The receiver never
makes that API call itself — auth + transport stay in the proxy
layer.

Users can also bypass the proxy by typing the id directly:

```
/dismiss reason:"sanitizer added upstream" finding:untestedExport@src/auth/login.ts:loginUser#abc
```

Either path lands the same suppression entry.

## Why no built-in HTTP client?

Terrain's contract: zero outbound network calls. The binary writes
files / stdout; your workflow handles auth and transport. Three
reasons:

1. **Air-gapped friendly.** Adopters running in restricted networks
   (regulated industries, sovereign clouds, isolated CI runners) can
   use the same binary you do.
2. **No token lifetime in the binary.** `GITHUB_TOKEN` lives in the
   workflow runner for the duration of the job and disappears. A
   long-lived terrain process never sees it.
3. **Determinism.** A failing `gh api` call is a workflow failure,
   not a hidden retry inside terrain. Operators see the exact HTTP
   error in the action log.

The two-output split (`terrain pr --json` + `terrain report
check-runs`) is deliberate — one analyze pass, multiple consumers.

## What lives where

| Concern | Where |
|---|---|
| Analyze + render | `terrain` binary |
| Auth (`GITHUB_TOKEN`) | Workflow runner / receiver environment |
| HTTP transport | `gh api` (workflow) / proxy (webhook reply) |
| Webhook signature validation | `terrain webhook` |
| Slash-command dispatch | `terrain webhook` |
| Suppression file writes | `terrain webhook` → `.terrain/suppressions.yaml` |
| Finding-history learning | `terrain analyze` → `.terrain/finding-history.yaml` |
