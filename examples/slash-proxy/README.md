# slash-proxy

Reference glue between GitHub webhook deliveries and the
`terrain webhook` receiver.

Terrain itself does no outbound HTTP. This shell script forwards
incoming webhooks to the receiver and re-posts the reply back to
the PR thread.

See `docs/integrations/github-checks.md` for the deploy story.

## Files

| File | Purpose |
|---|---|
| `proxy.sh` | Stdin reads the raw webhook body; posts back via `gh api` |
| `Dockerfile` | Minimal image bundling `proxy.sh` + a tiny HTTP server |

Production deployments will likely replace `proxy.sh` with whatever
HTTP framework / serverless runtime fits the host. The contract is
narrow: forward → take reply → POST to issues/comments.
