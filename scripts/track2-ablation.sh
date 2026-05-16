#!/usr/bin/env bash
# Track 2 — synthetic ablation experiment.
#
# Goal: prove Terrain's Tier 2 (observability) detectors fire on the
# structural conditions they claim to fire on. The ablation creates a
# known structural change in a real repo; Terrain must detect it.
#
# Method:
#   1. Baseline:  `terrain analyze <repo>` → record signal counts
#   2. Ablation:  apply a structural change (rm eval, add prompt, etc.)
#   3. Test:      re-run analyze, diff against baseline
#   4. Restore:   undo the ablation, confirm baseline restored
#
# Each ablation has a PREDICTION about what signals should change.
# Match = causal evidence the detector is doing what it claims.

set -uo pipefail
cd "$(dirname "$0")/.."

REPO="${REPO:-/tmp/scan-validate/repos/promptfoo}"
TERRAIN="${TERRAIN:-/tmp/terrain}"
OUT_DIR="tier-4/track2-ablation"
mkdir -p "$OUT_DIR"

log() { echo "[$(date +%H:%M:%S)] [track2] $*"; }

# Build a fresh terrain binary.
log "build terrain"
go build -o "$TERRAIN" ./cmd/terrain

# Stage 0: baseline.
log "stage 0: baseline analyze on $REPO"
"$TERRAIN" analyze --root "$REPO" --write-snapshot >/dev/null 2>&1
cp "$REPO/.terrain/snapshots/latest.json" "$OUT_DIR/baseline-snapshot.json"
baseline_counts=$(jq -c '.signals | group_by(.type) | map({(.[0].type): length}) | add' "$OUT_DIR/baseline-snapshot.json")
log "  baseline signal counts: $baseline_counts"

# ablation_run: runs analyze, captures signal counts vs baseline, writes diff.
ablation_run() {
  local name="$1"
  log "  re-analyzing after $name..."
  "$TERRAIN" analyze --root "$REPO" --write-snapshot >/dev/null 2>&1
  cp "$REPO/.terrain/snapshots/latest.json" "$OUT_DIR/$name-snapshot.json"

  local after_counts
  after_counts=$(jq -c '.signals | group_by(.type) | map({(.[0].type): length}) | add' "$OUT_DIR/$name-snapshot.json")
  log "    after counts: $after_counts"

  # Diff: which signal types changed count?
  log "    delta vs baseline:"
  jq -n --argjson b "$baseline_counts" --argjson a "$after_counts" \
    '($a | to_entries) as $aE
     | ($b | to_entries) as $bE
     | ($aE + $bE | group_by(.key) | map({
         key: .[0].key,
         delta: ((map(.value) | add) - (map(.value) | min) * 2)
       }) | map(select(.delta != 0)))' \
    | jq -r '.[] | "      \(.key): \(.delta)"' 2>&1
}

# === Ablation 1: delete a promptfooconfig.yaml (remove eval coverage) ===
log ""
log "== Ablation 1: delete one promptfooconfig.yaml =="
log "  PREDICTION: surfaceMissingEval count goes UP (formerly-covered surfaces now uncovered)"
TARGET_EVAL=$(find "$REPO" -name "promptfooconfig.yaml" -not -path "*/node_modules/*" -not -path "*/test/*" 2>/dev/null | head -1)
log "  ablating: $TARGET_EVAL"
if [[ -n "$TARGET_EVAL" ]] && [[ -f "$TARGET_EVAL" ]]; then
  cp "$TARGET_EVAL" "$OUT_DIR/saved-eval.yaml"
  rm "$TARGET_EVAL"
  ablation_run "ablation1-rm-eval"
  cp "$OUT_DIR/saved-eval.yaml" "$TARGET_EVAL"
  log "  restored"
else
  log "  SKIP — no candidate eval found outside node_modules/test"
fi

# === Ablation 2: add an LLM call site without eval ===
log ""
log "== Ablation 2: add new LLM call site (uncovered surface) =="
log "  PREDICTION: surfaceMissingEval count goes UP (new uncovered AI surface)"
ABLATION_FILE="$REPO/src/ablation-test-callsite.ts"
cat > "$ABLATION_FILE" << 'TS'
// Track 2 ablation: brand-new LLM call site with no eval coverage.
// Terrain should flag this file as surfaceMissingEval.
import OpenAI from 'openai';

export async function ablationCall(prompt: string): Promise<string> {
  const client = new OpenAI({ apiKey: process.env.OPENAI_API_KEY! });
  const response = await client.chat.completions.create({
    model: 'gpt-4',
    messages: [{ role: 'user', content: prompt }],
  });
  return response.choices[0].message.content || '';
}
TS
ablation_run "ablation2-add-llm-callsite"
rm "$ABLATION_FILE"
log "  removed ablation file"

# === Ablation 3: add a prompt file without eval coverage ===
log ""
log "== Ablation 3: add new prompt file (uncovered surface) =="
log "  PREDICTION: surfaceMissingEval fires on the new prompt path"
ABLATION_PROMPT_DIR="$REPO/src/test-ablation-prompts"
mkdir -p "$ABLATION_PROMPT_DIR"
cat > "$ABLATION_PROMPT_DIR/uncovered.yaml" << 'YAML'
system_prompt: |
  You are a helpful assistant.
messages:
  - role: user
    content: "{{user_query}}"
YAML
ablation_run "ablation3-add-prompt"
rm -rf "$ABLATION_PROMPT_DIR"
log "  removed ablation prompts dir"

# === Ablation 4: pin a deprecated model ===
log ""
log "== Ablation 4: introduce a deprecated model pin =="
log "  PREDICTION: aiModelDeprecationRisk fires on the new file"
ABLATION_DEP_FILE="$REPO/src/ablation-deprecated.ts"
cat > "$ABLATION_DEP_FILE" << 'TS'
// Track 2 ablation: deprecated model pin.
// Terrain's aiModelDeprecationRisk detector should flag this.
export const MODEL_CONFIG = {
  model: 'gpt-3.5-turbo',  // deprecated tag
  temperature: 0.7,
};
TS
ablation_run "ablation4-deprecated-model"
rm "$ABLATION_DEP_FILE"
log "  removed ablation deprecated-model file"

log ""
log "DONE. Snapshots in $OUT_DIR/"
ls -la "$OUT_DIR/"
