#!/usr/bin/env bash
# verify_doc_consistency.sh — guard against doc-drift between the
# manifest, the CLI surface, the README headline, and the rule docs.
#
# Run in CI alongside `make test`. Exits non-zero on any drift.
# Fast: pure-shell + grep; no Go build required.
#
# Adopted from cycle-2 plan P7.11.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

fail=0
fail() {
  echo "FAIL: $*" >&2
  fail=$((fail + 1))
}

# ── Check 1 — every manifest RuleURI exists on disk ────────────
# The Go test TestRuleDocs_ExistOnDisk also enforces this, but the
# shell check runs faster and catches the issue before tests run.
echo "[1/4] Checking manifest RuleURI references resolve to docs..."
while IFS= read -r ruleURI; do
  if [ ! -f "$ruleURI" ]; then
    fail "manifest RuleURI does not exist on disk: $ruleURI"
  fi
done < <(grep 'RuleURI:' internal/signals/manifest.go \
         | sed 's|.*"\(docs/rules/[^"]*\)".*|\1|' \
         | sort -u)

# ── Check 2 — README + DESIGN + OVERVIEW + PRODUCT mention LLM-free ──
echo "[2/4] Checking positioning consistency across user-facing docs..."
for doc in README.md DESIGN.md docs/OVERVIEW.md docs/PRODUCT.md; do
  if ! grep -qi -E 'no.*api key|llm-free|local.first|locally|local-only' "$doc"; then
    fail "$doc does not mention the no-API-key / LLM-free positioning"
  fi
done

# ── Check 3 — every prtemplates entry's signal_type has a manifest entry ──
echo "[3/4] Checking prtemplates entries point at real signal types..."
while IFS= read -r sig_type; do
  if ! grep -q "models\\.SignalType = \"${sig_type}\"" internal/signals/signal_types.go; then
    fail "prtemplates signal_type has no signal_types.go constant: ${sig_type}"
  fi
done < <(grep '  - signal_type:' internal/prtemplates/templates.yaml \
         | awk '{print $3}' \
         | sort -u)

# ── Check 4 — no internal-vocabulary leaks in public surfaces ──
echo "[4/4] Checking public surfaces for internal-vocabulary leaks..."
# These tokens commonly leak from internal planning docs; the privacy
# memory and prior scrubs forbid them in public-visible files.
forbidden_in_public='\b(Slice [0-9]|S2 surface|S2 pipeline|round-[0-9] review|adversarial review|n>=[0-9]+|Audit-named gap|framework framework_migration|§[0-9])'
public_globs=(
  'README.md'
  'DESIGN.md'
  'CHANGELOG.md'
  'docs/PRODUCT.md'
  'docs/OVERVIEW.md'
  'docs/cli-spec.md'
  'docs/quickstart.md'
)
for f in "${public_globs[@]}"; do
  if [ -f "$f" ] && grep -E "$forbidden_in_public" "$f" >/dev/null; then
    fail "$f contains an internal-vocabulary token (forbidden in public docs)"
    grep -nE "$forbidden_in_public" "$f" | head -3 | sed 's/^/    /' >&2
  fi
done

if [ "$fail" -gt 0 ]; then
  echo
  echo "Doc-consistency: $fail check(s) failed." >&2
  exit 1
fi
echo "Doc-consistency: all checks passed."
