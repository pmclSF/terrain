# Canary Set — Selection Criteria + Candidate Repos

A frozen list of 15-25 real PRs from real AI-product repos. Re-run terrain against each weekly, and after every detector change. This is the week-over-week benchmark for "is the panel still healthy?"

## Selection criteria

1. **App-shaped AI repos, not libraries.** Per saved memory `terrain_corpus_and_gate.md`: sample apps not stars. Real applications shipping AI features in production, not framework / SDK / example-collection repos.
2. **Language mix.** At least Python + TypeScript. Bonus weight for Go and Java if available.
3. **Merged, time-stable PRs.** Pick PRs that landed on `main` ≥30 days ago. Ground truth doesn't shift retroactively.
4. **Public / permissively licensed.** So validation results can be shared without legal cleanup.
5. **One PR per "detector category" we want to measure.** Coverage matrix below.
6. **Repository activity floor.** ≥1 commit/week median over the last 6 months. Dead repos drift; alive repos let us re-measure on a fresh state.

## Detector coverage matrix (15-PR floor)

The canary should exercise every gate-tier-candidate detector at least once. Target shape:

| Detector category | Example PR shape | Min PRs |
|---|---|---|
| Schema change × prompt consumer | PR renames a field on a Pydantic / TypeScript schema referenced in a prompt template | 2 |
| Model deprecation / drift | PR pins a deprecated model ID; OR PR bumps an LLM SDK without bumping the model string | 2 |
| Prompt drift | PR changes a prompt template's wording in a non-trivial way (≥3 line diff) | 2 |
| Eval coverage gap | PR adds a new prompt surface without an accompanying eval scenario | 2 |
| Hardcoded API key (test fixture) | PR adds a fake `sk-test-…` key in a test or example file (this is a near-miss, terrain should NOT fire here) | 1 |
| Hardcoded API key (real) | PR accidentally commits a real-shaped key (hand-seed via canary PR if absent in real repos) | 1 |
| Non-deterministic eval | PR adds a promptfoo / deepeval config without `temperature: 0` | 1 |
| Untested export | PR exports a new function with no test under the standard test path | 1 |
| Mock-heavy test / weak assertion | PR adds tests that mock 4+ dependencies or use only `toBeTruthy()` | 1 |
| Static skipped test | PR adds a `.skip` / `@pytest.mark.skip` without ticket reference | 1 |
| Floating model tag | PR uses `gpt-4` or `claude-3-opus` without date suffix | 1 |
| Negative / null cases | PRs that should produce ZERO findings — pure refactors, doc fixes, dependency bumps with no model implications | 3 |

Total floor: 18 PRs. Cushion to 22 for resilience to 1-2 dropping out.

## Candidate repo categories (you select)

The actual PR URLs go in `tier-4/canary-set.yaml` once curated. Below are the categories I'd target. **You should pick the repos** — I can do web research and propose specific candidates in a follow-up if helpful, but you have the strongest intuition for which AI repos are "app-shaped" vs example-collection-shaped.

**Category A: Production RAG / chatbot apps (Python, FastAPI / Flask)**
Look for repos with: `main.py` or `app.py` that mounts an LLM endpoint, a `prompts/` directory with `.md` or `.j2` templates, an `evals/` or `tests/eval_*` directory, a `requirements.txt` pinning `anthropic`, `openai`, or `langchain`. Anti-shape: pure LangChain / LlamaIndex tutorials, "awesome-X" collections.

**Category B: AI-feature-shipping SaaS frontends (TypeScript, Next.js)**
Look for: `app/api/chat/route.ts` (or similar AI-SDK endpoint), `lib/prompts/`, eval config in `package.json` scripts, `@anthropic-ai/sdk` or `openai` in dependencies. Anti-shape: AI-SDK starter templates, Vercel example demos.

**Category C: Classical ML pipelines with prompt-augmented steps (Python, dbt + sklearn)**
Look for: `dbt_project.yml` + ML training scripts + prompt-template files. Mix of structural ML and AI hybrid is interesting because it exercises cross-domain detectors.

**Category D: Agent / tool-using apps (Python or TypeScript)**
Look for: `tools.yaml` or equivalent agent-definition files, destructive tool definitions, MCP-style structures. Exercises `aiToolWithoutSandbox` and related.

**Category E: Promptfoo / DeepEval / Ragas adopters**
Repos that have committed eval configurations. Exercises the eval-framework integration.

**Diversity targets:**
- ≥2 categories from above
- ≥2 languages (Python + TypeScript at minimum)
- ≥3 organizations / authors (avoid all-one-author bias)
- 1 dogfood-eligible (terrain itself or a repo we've already labeled)

## Per-PR record schema

When the canary set is sealed, store each PR as:

```yaml
# tier-4/canary-set.yaml
schema_version: 1
sealed_at: 2026-MM-DD
prs:
  - id: canary-001
    repo: github.com/<org>/<repo>
    pr_number: 1234
    pr_url: https://github.com/<org>/<repo>/pull/1234
    merged_at: 2026-MM-DD
    head_sha: abc123…           # frozen; re-checkout this exact tree
    base_sha: def456…           # diff base
    category: schema-change-prompt-consumer
    languages: [python, typescript]
    expected_findings:           # ground truth
      - rule_id: schemaChangePromptConsumer
        path: src/prompts/order.md
        verdict: TP
      - rule_id: aiNonDeterministicEval
        path: evals/order.yaml
        verdict: TP
    expected_non_findings:       # what terrain should NOT fire
      - rule_id: aiHardcodedAPIKey
        why: test fixture, key matches sk-fake-* allowlist
    notes: brief description of why this PR is in the canary
```

## Weekly re-run

A `make canary` target runs terrain against every PR's frozen `head_sha`, diffs against `expected_findings`, and emits a per-PR pass/fail. Thresholds:

- **Hard fail:** any expected TP that terrain misses (recall regression)
- **Soft fail:** any expected non-finding that terrain now fires (precision regression)
- **Pass:** all expected findings + non-findings match

Per the saved binding rules (`feedback_no_single_file_rules`, `feedback_detector_fixes`), the canary is a *recall regression gate*, not a precision-floor gate. Precision-floor work is the validation-corpus job, not the canary job.

## Sealing process

1. You select 5-10 candidate repos per category above.
2. I (or you) browse the merged-PR history of each, looking for PRs that match the coverage matrix.
3. We assemble `tier-4/canary-set.yaml` with 18-22 PRs.
4. First `make canary` run establishes the "expected findings" ground truth. Any disagreement is hand-resolved.
5. Sealed. Re-run weekly.

## Status

- Criteria: drafted (this doc).
- Candidate repos: **awaiting your selection** — propose categories above or candidate URLs.
- Sealed canary file (`tier-4/canary-set.yaml`): not yet created.
- `make canary` target: not yet wired.

Once you give me 5-10 candidate repos (or ask me to research them), the rest is mechanical.
