# Pinned dependencies — security review notes

Terrain pins every Go and npm dependency to a specific version (Go
modules use `v0.0.0-<date>-<commit>` style refs for non-tagged
upstreams; npm uses caret-locked entries plus `package-lock.json`).
This page documents the dependencies whose pinning was deliberate
beyond the standard "latest stable" reflex.

CI surfaces drift via Dependabot (npm + gomod + github-actions
ecosystems). PRs that bump a pinned dependency below should be
reviewed against the rationale in this file rather than rubber-
stamped.

## Tree-sitter grammars

`github.com/smacker/go-tree-sitter` provides the parser bindings for
JS, TS, Python, and Java AST extraction. The pinned commit must be
verified against:

- The upstream Tree-sitter grammar repos for each language. The
  smacker bindings vendor a specific snapshot of each grammar; any
  change to that snapshot can shift parser behaviour, even within a
  same-language upgrade.
- CGO toolchain compatibility. Tree-sitter requires a C compiler at
  build time; new bindings revisions occasionally bump the minimum
  C-language standard.

Active grammars (one entry per `smacker/go-tree-sitter/...` import in
the Terrain tree):

| Grammar | Purpose | Files |
|---|---|---|
| `javascript` | JS/JSX test extraction + conversion | `internal/testcase/ast_javascript.go`, `internal/convert/js_ast.go` |
| `typescript/typescript` | TS/TSX test extraction + conversion | same as above |
| `python` | pytest / unittest extraction + conversion | `internal/testcase/ast_python.go` |
| `java` | JUnit 4/5 / TestNG extraction | `internal/testcase/ast_java.go` |

When a Dependabot PR proposes a tree-sitter bump, run:

```bash
go test ./internal/testcase/... ./internal/convert/...
```

against a calibration fixture set that exercises every grammar. The
existing `make calibrate` target is one entry point; expand the
fixture set if a grammar's coverage is light.

## YAML parser

`gopkg.in/yaml.v3` parses eval configs, agent definitions, and the
calibration `labels.yaml` schema. Pinned to the v3 line because the
v3 → v4 migration changed default behaviours (escaping, anchor
handling) that would break existing fixtures.

## NPM lockfile policy

`package-lock.json` is committed and verified by CI's `npm-package`
job. Any drift between `package.json` and the lockfile fails the gate
— the explicit message is "run `npm install` locally and commit the
updated lockfile". Same contract holds for `extension/vscode/package-lock.json`.

## Cosign / Sigstore

The npm postinstaller uses cosign for keyless signature verification.
Cosign itself is not a Go module dependency — it's installed on the
host. The release pipeline pins `cosign-installer@v3` (via SHA in
`.github/workflows/release.yml`) so the verification chain is fixed
at the workflow level.

## When in doubt

If a Dependabot PR has no clear story in this file, comment-block the
PR with the rationale you discover. Future bumps reference this file
as the audit trail.
