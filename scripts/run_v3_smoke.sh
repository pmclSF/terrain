#!/usr/bin/env bash
# Runs the v3 matched-stimulus smoke:
#   step 1: re-rate Claude on 20 stratified rows with v3 prompt
#   step 2: cross-rate OpenAI on the SAME 20 rows with v3 prompt
# Output files live under tier-4/.
set -euo pipefail
cd "$(dirname "$0")/.."

RULES="deprecatedTestPattern,aiNonDeterministicEval,aiSafetyEvalMissing,uncoveredAISurface,promptFileMissingEval"
CLAUDE_OUT="tier-4/detector-validation-v3-claude-smoke.jsonl"
OPENAI_OUT="tier-4/detector-validation-v3-openai-smoke.jsonl"

echo "=== Step 1: Claude v3 re-rate (20 rows, ~3 min) ==="
python3 scripts/rerate_claude_with_prompt.py \
    --rules "$RULES" \
    --max-rows 20 \
    --prompt-version v3-anti-anchor \
    --out "$CLAUDE_OUT"

echo
echo "=== Step 2: OpenAI v3 cross-rate on same 20 rows (~1 min, ~\$0.30) ==="
if [[ -z "${OPENAI_API_KEY:-}" ]]; then
    echo "ERROR: OPENAI_API_KEY not set; export it and re-run." >&2
    exit 2
fi
.venv/bin/python scripts/cross_rate_openai.py \
    --in "$CLAUDE_OUT" \
    --prompt-version v3-anti-anchor \
    --model gpt-4o \
    --out "$OPENAI_OUT" \
    --stats-every 10

echo
echo "=== Done ==="
echo "Claude verdicts (v3): $CLAUDE_OUT"
echo "OpenAI verdicts (v3): $OPENAI_OUT"
