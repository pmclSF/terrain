#!/usr/bin/env bash
# scripts/docs-verify.sh
#
# Companion to `cmd/terrain-docs-gen`. Regenerates every doc the
# generator owns into a temp directory and diffs against the committed
# copy on disk. Exits non-zero on any drift, suitable for use as a
# CI gate.
#
# Three sets of files are checked:
#   1. docs/signals/manifest.json
#   2. docs/severity-rubric.md
#   3. docs/rules/*.md — only the stub block above the
#      "<!-- docs-gen: end stub. -->" marker is diffed; anything below
#      the marker is hand-authored and preserved.
#
# Run via `make docs-verify`. Lives outside the Makefile because GNU
# make strips in-recipe `#` comments and breaks `\`-chained recipes.

set -euo pipefail

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

go run ./cmd/terrain-docs-gen -out "$tmp" >/dev/null

rc=0

# 1. Top-level deterministic outputs.
for f in docs/signals/manifest.json docs/severity-rubric.md ; do
  if ! diff -u "$f" "$tmp/$f" ; then
    echo "::error::$f is out of date. Run 'make docs-gen' and commit." >&2
    rc=1
  fi
done

# 2. Generated rule-doc stubs.
marker='<!-- docs-gen: end stub.'

if [ -d docs/rules ] ; then
  while IFS= read -r f ; do
    rel="docs/rules/${f#./}"
    gen="$tmp/$rel"
    if [ ! -f "$gen" ] ; then
      echo "::error::$rel exists on disk but the generator wouldn't produce it. Remove it or add the corresponding manifest entry." >&2
      rc=1
      continue
    fi
    gen_stub="$(awk -v m="$marker" 'index($0,m){exit} {print}' "$gen")"
    cur_stub="$(awk -v m="$marker" 'index($0,m){exit} {print}' "$rel")"
    if [ "$gen_stub" != "$cur_stub" ] ; then
      echo "::error::$rel stub block is out of sync with the manifest. Run 'make docs-gen' and commit." >&2
      rc=1
    fi
  done < <(cd docs/rules && find . -name '*.md' | sort)
fi

if [ "$rc" -ne 0 ] ; then
  exit "$rc"
fi
echo "docs-verify: manifest.json + severity-rubric.md + docs/rules/* stub blocks are up to date."
