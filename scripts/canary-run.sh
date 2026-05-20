#!/usr/bin/env bash
# scripts/canary-run.sh — drive the canary set against the local terrain binary.
#
# Reads harness/canary/canary-set.yaml. For each entry with a populated head_sha + base_sha
# (i.e. sealed), the script:
#   1. Clones the upstream repo at the head_sha into a working dir
#   2. Runs `terrain analyze` (or the equivalent `terrain ai findings`)
#   3. Diffs the emitted findings against `expected_findings` + `expected_non_findings`
#   4. Reports per-PR pass / fail
#
# For draft entries (no head_sha), the script prints a clear status message and skips.
# For HAND-SEED-REQUIRED entries (canary-019, canary-020), the script prints the seed
# instructions and skips until the user has populated them.
#
# Exit codes:
#   0 — all sealed entries pass
#   1 — at least one expected TP missed (recall regression — HARD FAIL)
#   2 — at least one expected non-finding fired (precision regression — SOFT FAIL)
#   3 — set is not sealed yet (only draft entries present)
#   4 — usage / config error

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CANARY_FILE="${ROOT}/harness/canary/canary-set.yaml"
WORK_DIR="${ROOT}/.terrain/canary-work"
REPORT_FILE="${ROOT}/.terrain/canary-report.json"

mkdir -p "${WORK_DIR}" "$(dirname "${REPORT_FILE}")"

if [ ! -f "${CANARY_FILE}" ]; then
  echo "error: ${CANARY_FILE} not found" >&2
  exit 4
fi

# Resolve the terrain binary to use. Prefer freshly built /tmp/terrain if present.
TERRAIN_BIN="${TERRAIN_BIN:-}"
if [ -z "${TERRAIN_BIN}" ]; then
  if [ -x "/tmp/terrain" ]; then
    TERRAIN_BIN="/tmp/terrain"
  elif command -v terrain >/dev/null 2>&1; then
    TERRAIN_BIN="$(command -v terrain)"
  else
    echo "error: terrain binary not found. Set TERRAIN_BIN or build with 'go build -o /tmp/terrain ./cmd/terrain'" >&2
    exit 4
  fi
fi

echo "canary harness — driving ${TERRAIN_BIN}"
echo "canary set:    ${CANARY_FILE}"
echo "work dir:      ${WORK_DIR}"
echo "report:        ${REPORT_FILE}"
echo

# Use Python for the YAML walk + per-entry driver — pure-bash YAML parsing isn't worth it.
exec python3 "${ROOT}/scripts/canary-driver.py" \
  --canary-file "${CANARY_FILE}" \
  --work-dir "${WORK_DIR}" \
  --report-file "${REPORT_FILE}" \
  --terrain-bin "${TERRAIN_BIN}" \
  "$@"
