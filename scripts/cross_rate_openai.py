#!/usr/bin/env python3
"""
OpenAI cross-rate against the v2 Claude validation baseline.

For each row in tier-4/detector-validation-v2-combined-good.jsonl,
ask OpenAI the SAME prompt Claude saw (verbatim) and compute Cohen's
kappa per detector. The result tells us which Phase-3 graduation
verdicts rest on a single rater.

Usage:
    OPENAI_API_KEY=sk-... python3 scripts/cross_rate_openai.py \
        --in tier-4/detector-validation-v2-combined-good.jsonl \
        --out tier-4/detector-validation-v2-openai.jsonl \
        --model gpt-4o-mini \
        --rules deprecatedTestPattern,aiNonDeterministicEval \
        --max-rows 20 \
        --concurrency 5 \
        --max-cost-usd 5.0

Resume: rows whose dedup key already appears in --out AND whose
`_openai_model` matches `--model` are skipped. UNK (parse/api error)
rows are NOT persisted, so resume retries them.

Output: each row is the input plus
    _openai_verdict: {"verdict": "TP"|"FP"|"UNCERTAIN", "reason": "..."}
    _openai_model:   the model used
    _prompt_version: prompt revision tag

Cost (approx): gpt-4o-mini at ~700 input + 80 output tokens per row:
    1000 rows  →  ~$0.16
    3200 rows  →  ~$0.50
gpt-4o is ~30x more expensive; gpt-4-turbo ~60x.
"""
import argparse
import hashlib
import json
import os
import random
import re
import sys
import threading
import time
from collections import Counter, defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path


# Verbatim copy of validate_detectors_claude.py's PROMPT_TEMPLATE.
# Use this only to compare against the existing Claude verdicts in
# tier-4/detector-validation-v2-combined-good.jsonl.
PROMPT_V2_CLAUDE_FAITHFUL = """\
You are validating findings emitted by Terrain, a static AI/ML code-quality
analyzer. For each finding, judge whether the detector correctly identified
a real instance (TP) or fired on something unrelated (FP).

DETECTOR RULE: {rule_id}
SEVERITY:      {severity}
TITLE:         {title}
DESCRIPTION:   {description}

REPO:          {repo}
FILE:          {file}
EVIDENCE:      {evidence}

A few lines of context from the file (HEAD revision):
{file_excerpt}

Respond with EXACTLY one JSON object, no markdown, no prose:
{{"verdict":"TP","reason":"<one short sentence>"}}

Use "TP" if the rule correctly identified the thing it claims to detect.
Use "FP" if the rule mis-fired (docstring, comment, test fixture, vendor,
wrong file shape, false negative inversion, etc.).
Use "UNCERTAIN" only if the record + excerpt are genuinely insufficient.
"""

# Anti-rubber-stamp prompt. Forces file-kind classification BEFORE the
# rater sees the detector's claim, then a 3-step verification (locate,
# scope, claim). Designed against the v2-smoke failure mode where 4o
# accepted Claude-FPs because the file/symbol kind didn't match the rule.
# Must be paired with a Claude re-rate using the same prompt for kappa.
PROMPT_V3_ANTI_ANCHOR = """\
You are auditing a detector finding from Terrain, a static AI/ML code-quality
tool. Detectors fire on lexical patterns and sometimes misclassify. Verify,
against the file contents, whether the detector's claim holds up structurally.
Default to skepticism: if the symbol isn't visible as a real code construct,
or the file kind is out of scope for the rule, the finding is FP.

# Step 1: read the file, ignoring the detector's claim for now

FILE: {file}
FILE EXCERPT (may be truncated):
{file_excerpt}

What kind of file is this? Pick ONE letter:
  (a) production source — real call sites, business logic
  (b) test or fixture — mocks, asserts, expected values
  (c) demo / example / cookbook / notebook — showcase, not production
  (d) configuration — yaml/json/toml declarative settings
  (e) eval results / output artifact — OUTPUT of a previous run, not the run
  (f) schema / type definition — Pydantic, TypedDict, .d.ts, dataclass
  (g) dataset / data generation script — produces data, not behavior
  (h) README / doc / markdown / changelog

# Step 2: now read the detector's claim

RULE:     {rule_id}
TITLE:    {title}
SYMBOL:   {symbol}
EVIDENCE: {evidence}

# Step 3: verify, in order

1. LOCATE: Find `{symbol}` in the excerpt as a real code construct
   (function/class definition, import, call site, or assignment).
   Symbols appearing ONLY in comments, docstrings, log messages,
   example string values, JSON keys, or list items DO NOT count.

2. SCOPE: Is the detector's claim meaningful for THIS file kind?
   Common scope errors:
   - "missing eval coverage" rules do not apply to (g) datasets,
     (c) demos, or (f) schemas — the file isn't a target of evaluation.
   - "non-deterministic eval provider" rules want eval-harness config,
     not (d) production runtime config nor (e) result artifacts.
   - "uncovered AI surface" rules require the SYMBOL to be an AI model,
     prompt, or agent — not a chunking utility, HTTP handler, DB
     connector, math function, transcription model, or schema field.
   - "deprecated test pattern" rules require the deprecated call to be
     actually visible in the excerpt; if the excerpt does not contain
     it, you cannot confirm.

3. CLAIM: If locate and scope pass, is the detector's claim actually
   true against the visible code (e.g., is there really no eval, no
   test, no safety check, no temperature setting)?

# Step 4: verdict

TP        — locate Y, scope Y, claim Y
FP        — locate N OR scope N OR claim contradicted by visible code
UNCERTAIN — excerpt truncation prevents verifying locate or scope

Output EXACTLY one JSON object on a single line, no markdown:
{{"verdict":"TP","reason":"<kind letter> | locate=Y | scope=Y | <short why>"}}
or {{"verdict":"FP","reason":"<kind letter> | locate=N | <short why>"}}
or {{"verdict":"UNCERTAIN","reason":"<what truncation prevents verifying>"}}
"""

PROMPT_VERSIONS = {
    "v2-claude-faithful": PROMPT_V2_CLAUDE_FAITHFUL,
    "v3-anti-anchor": PROMPT_V3_ANTI_ANCHOR,
}

SYSTEM_MSG = "You output only a single JSON object on one line."

# USD per 1M tokens. Public list pricing as of writing; update as needed.
COST_USD_PER_1M = {
    "gpt-4o-mini":  {"in": 0.15,  "out": 0.60},
    "gpt-4o":       {"in": 5.00,  "out": 15.00},
    "gpt-4-turbo":  {"in": 10.00, "out": 30.00},
    "gpt-4":        {"in": 30.00, "out": 60.00},
}

JSON_RE = re.compile(r"\{[^{}]*\"verdict\"[^{}]*\}", re.DOTALL)


# ---------- Row helpers ----------

def build_prompt(row, prompt_version):
    tpl = PROMPT_VERSIONS[prompt_version]
    return tpl.format(
        rule_id=row.get("rule_id", ""),
        severity=row.get("severity", "(unknown)"),
        title=row.get("title", "(no title)"),
        description=(row.get("description", "(no description)") or "")[:400],
        repo=row.get("repo", ""),
        file=row.get("file", "(no file)"),
        symbol=row.get("symbol", "") or "(none)",
        evidence=(row.get("evidence", "(no evidence)") or "")[:500],
        file_excerpt=(row.get("_file_excerpt", "") or "")[:2500],
    )


def evidence_hash(row):
    h = hashlib.sha1((row.get("evidence", "") or "").encode("utf-8", errors="replace"))
    return h.hexdigest()[:8]


def key_of(row):
    """Dedup key. Includes evidence-hash so multiple fires on the same
    (repo, rule_id, file, symbol) at different lines don't collide."""
    return (row.get("repo", ""), row.get("rule_id", ""),
            row.get("file", ""), row.get("symbol", ""),
            evidence_hash(row))


def claude_verdict_of(row):
    return (row.get("_verdict") or {}).get("verdict")


def estimate_tokens(text):
    # Cheap heuristic — char/4. Off by ~10-20% vs tiktoken; fine for cost cap.
    return max(1, len(text) // 4)


# ---------- IO ----------

def load_rows(path):
    rows = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError as e:
                sys.stderr.write(f"skip malformed input row: {e}\n")
    return rows


def load_already_rated(path, expected_model, expected_prompt_version):
    """Keys already rated by the SAME (model, prompt_version) pair.
    A change in either forces re-rate."""
    seen = set()
    if not os.path.exists(path):
        return seen
    with open(path) as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            if r.get("_openai_model") != expected_model:
                continue
            if r.get("_prompt_version") != expected_prompt_version:
                continue
            v = (r.get("_openai_verdict") or {}).get("verdict")
            if v in ("TP", "FP", "UNCERTAIN"):
                seen.add(key_of(r))
    return seen


# ---------- Verdict parsing ----------

def parse_verdict(text):
    text = (text or "").strip()
    if text.startswith("```"):
        # Strip fenced-code block.
        text = text.split("\n", 1)[1] if "\n" in text else text
        if text.endswith("```"):
            text = text.rsplit("```", 1)[0]
        text = text.strip()
    try:
        obj = json.loads(text)
        if isinstance(obj, dict) and "verdict" in obj:
            return obj
    except json.JSONDecodeError:
        pass
    m = JSON_RE.search(text)
    if m:
        try:
            return json.loads(m.group(0))
        except json.JSONDecodeError:
            pass
    return {"verdict": "UNK", "reason": "parse-fail", "raw_head": text[:120]}


# ---------- OpenAI call ----------

class FatalError(Exception):
    """Unrecoverable — bail the whole run."""


def call_openai(row, model, client, prompt_version, max_retries=3):
    """Returns (verdict_dict, tokens_in_est, tokens_out_est).
    Raises FatalError on auth failures."""
    prompt = build_prompt(row, prompt_version)
    tokens_in = estimate_tokens(SYSTEM_MSG) + estimate_tokens(prompt)

    last_err = None
    for attempt in range(max_retries):
        try:
            resp = client.chat.completions.create(
                model=model,
                messages=[
                    {"role": "system", "content": SYSTEM_MSG},
                    {"role": "user", "content": prompt},
                ],
                temperature=0.0,
                max_tokens=200,
            )
            text = resp.choices[0].message.content or ""
            tokens_out = estimate_tokens(text)
            return parse_verdict(text), tokens_in, tokens_out
        except Exception as e:
            msg = str(e)
            cls = type(e).__name__
            # Auth: don't burn retries.
            if "401" in msg or "AuthenticationError" in cls or "Incorrect API key" in msg:
                raise FatalError(f"auth: {msg}")
            # Rate limit: long backoff.
            is_429 = "429" in msg or "RateLimit" in cls
            base = 10 if is_429 else 2
            last_err = e
            if attempt < max_retries - 1:
                time.sleep(base * (2 ** attempt))
    return ({"verdict": "UNK", "reason": f"openai error after {max_retries}: {last_err}"},
            tokens_in, 0)


# ---------- Stats ----------

def cohens_kappa(a, b):
    if len(a) != len(b) or not a:
        return 0.0
    n = len(a)
    agree = sum(1 for x, y in zip(a, b) if x == y)
    po = agree / n
    c1 = Counter(a)
    c2 = Counter(b)
    pe = sum((c1[k] / n) * (c2[k] / n) for k in set(list(c1) + list(c2)))
    if pe >= 0.9999:
        return 1.0 if po >= 0.9999 else 0.0
    return (po - pe) / (1 - pe)


def kappa_2class(claude_list, openai_list):
    """Kappa restricted to rows where both raters said TP or FP."""
    a, b = [], []
    for c, o in zip(claude_list, openai_list):
        if c in ("TP", "FP") and o in ("TP", "FP"):
            a.append(c); b.append(o)
    return cohens_kappa(a, b) if a else 0.0, len(a)


def per_detector_stats(claude_by_key, openai_by_key):
    by_det = defaultdict(lambda: ([], []))
    for key, cv in claude_by_key.items():
        ov = openai_by_key.get(key)
        if ov is None:
            continue
        det = key[1]
        by_det[det][0].append(cv)
        by_det[det][1].append(ov)
    return by_det


def print_stats_table(by_det, header, stream=sys.stderr):
    stream.write(f"\n{header}\n")
    stream.write(f"{'detector':30s} {'n':>4s} {'k3':>6s} {'k2':>6s} {'n2':>4s} {'agree':>6s}\n")
    for det in sorted(by_det):
        c, o = by_det[det]
        k3 = cohens_kappa(c, o)
        k2, n2 = kappa_2class(c, o)
        agree = sum(1 for x, y in zip(c, o) if x == y) / max(1, len(c))
        stream.write(f"{det:30s} {len(c):4d} {k3:6.3f} {k2:6.3f} {n2:4d} {100*agree:5.1f}%\n")


# ---------- Sampling ----------

def stratified_sample(rows, n_target, key_fn, seed=42):
    rng = random.Random(seed)
    buckets = defaultdict(list)
    for r in rows:
        buckets[key_fn(r)].append(r)
    per = max(1, n_target // max(1, len(buckets)))
    out = []
    for v in buckets.values():
        out.extend(rng.sample(v, min(per, len(v))))
    if len(out) < n_target:
        in_out = {id(r) for r in out}
        rest = [r for r in rows if id(r) not in in_out]
        rng.shuffle(rest)
        out.extend(rest[:n_target - len(out)])
    rng.shuffle(out)
    return out[:n_target]


# ---------- Main ----------

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--in", dest="inp",
                    default="tier-4/detector-validation-v2-combined-good.jsonl")
    ap.add_argument("--out",
                    default="tier-4/detector-validation-v2-openai.jsonl")
    ap.add_argument("--model", default="gpt-4o-mini",
                    help="OpenAI model id. gpt-4o-mini is the sweet spot.")
    ap.add_argument("--prompt-version", default="v3-anti-anchor",
                    choices=sorted(PROMPT_VERSIONS.keys()),
                    help="v2-claude-faithful matches the existing v2 corpus prompt verbatim; "
                         "v3-anti-anchor forces file-kind + locate + scope verification. "
                         "v3 requires Claude to be re-rated with the same prompt for kappa.")
    ap.add_argument("--max-rows", type=int, default=0,
                    help="0 = all rows after dedup; >0 = stratified sample of N rows across detectors")
    ap.add_argument("--rules", default="",
                    help="comma-separated rule_id allowlist; empty = all")
    ap.add_argument("--concurrency", type=int, default=5,
                    help="parallel OpenAI requests")
    ap.add_argument("--max-cost-usd", type=float, default=10.0,
                    help="bail when running cost estimate exceeds this")
    ap.add_argument("--stats-every", type=int, default=50,
                    help="print running per-detector kappa every N rated rows")
    ap.add_argument("--pacing-sec", type=float, default=0.0,
                    help="sleep N sec between dispatches (defensive against 429)")
    ap.add_argument("--max-retries", type=int, default=3)
    args = ap.parse_args()

    if not os.environ.get("OPENAI_API_KEY"):
        sys.exit("error: OPENAI_API_KEY not set")
    try:
        from openai import OpenAI
    except ImportError:
        sys.exit("error: pip install openai")
    client = OpenAI(api_key=os.environ["OPENAI_API_KEY"])

    rows = load_rows(args.inp)
    rule_filter = set(filter(None, [r.strip() for r in args.rules.split(",")]))
    if rule_filter:
        rows = [r for r in rows if r.get("rule_id") in rule_filter]

    seen = load_already_rated(args.out, args.model, args.prompt_version)
    todo = [r for r in rows if key_of(r) not in seen]

    if args.max_rows > 0 and len(todo) > args.max_rows:
        todo = stratified_sample(todo, args.max_rows, lambda r: r.get("rule_id", ""))

    sys.stderr.write(f"input rows (after rule filter): {len(rows)}\n")
    sys.stderr.write(f"already rated by ({args.model}, {args.prompt_version}): {len(seen)}\n")
    sys.stderr.write(f"to rate this run: {len(todo)}\n")
    sys.stderr.write(f"model={args.model} concurrency={args.concurrency} "
                     f"cost_cap=${args.max_cost_usd:.2f} prompt={args.prompt_version}\n")
    if args.prompt_version == "v3-anti-anchor":
        sys.stderr.write(
            "NOTE: v3 prompt diverges from the original Claude v2 prompt. The `_verdict` "
            "field in the input rows is on the v2 prompt; for valid kappa, feed this script "
            "input from a Claude-v3-rated JSONL (see scripts/rerate_claude_with_prompt.py).\n"
        )
    if args.model not in COST_USD_PER_1M:
        sys.stderr.write(f"WARN: no published price for {args.model}; cost estimates may be wrong\n")
    sys.stderr.write("\n")

    if not todo:
        sys.stderr.write("nothing to do; computing final stats from existing output\n")
    Path(os.path.dirname(args.out) or ".").mkdir(parents=True, exist_ok=True)

    cost_rates = COST_USD_PER_1M.get(args.model, {"in": 1.0, "out": 3.0})

    write_lock = threading.Lock()
    state = {
        "cost_usd": 0.0,
        "tokens_in": 0,
        "tokens_out": 0,
        "n_done": 0,
        "n_skipped_unk": 0,
        "fatal": None,
        "claude_v_by_key": {},
        "openai_v_by_key": {},
    }

    def worker(row):
        if state["fatal"]:
            return
        try:
            verdict, t_in, t_out = call_openai(row, args.model, client,
                                               args.prompt_version,
                                               max_retries=args.max_retries)
        except FatalError as e:
            with write_lock:
                state["fatal"] = str(e)
            return

        v = verdict.get("verdict")
        with write_lock:
            state["tokens_in"] += t_in
            state["tokens_out"] += t_out
            state["cost_usd"] = (state["tokens_in"] * cost_rates["in"]
                                 + state["tokens_out"] * cost_rates["out"]) / 1_000_000.0
            state["n_done"] += 1

            if v in ("TP", "FP", "UNCERTAIN"):
                out_row = dict(row)
                out_row["_openai_verdict"] = verdict
                out_row["_openai_model"] = args.model
                out_row["_prompt_version"] = args.prompt_version
                with open(args.out, "a") as f:
                    f.write(json.dumps(out_row) + "\n")
                    f.flush()
                key = key_of(row)
                cv = claude_verdict_of(row)
                if cv:
                    state["claude_v_by_key"][key] = cv
                state["openai_v_by_key"][key] = v
            else:
                # UNK: don't persist; resume will retry.
                state["n_skipped_unk"] += 1
                sys.stderr.write(
                    f"[unk] {row.get('rule_id','?')} {row.get('repo','?')}/"
                    f"{row.get('file','?')}: {str(verdict.get('reason',''))[:80]}\n"
                )

            if state["n_done"] % args.stats_every == 0:
                by_det = per_detector_stats(state["claude_v_by_key"],
                                            state["openai_v_by_key"])
                hdr = (f"=== running ({state['n_done']}/{len(todo)} rated, "
                       f"${state['cost_usd']:.3f} spent, "
                       f"{state['n_skipped_unk']} unk) ===")
                print_stats_table(by_det, hdr)

            if state["cost_usd"] > args.max_cost_usd:
                state["fatal"] = (f"cost cap ${args.max_cost_usd:.2f} exceeded "
                                  f"(${state['cost_usd']:.3f})")

    with ThreadPoolExecutor(max_workers=args.concurrency) as ex:
        futures = []
        for row in todo:
            if state["fatal"]:
                break
            futures.append(ex.submit(worker, row))
            if args.pacing_sec > 0:
                time.sleep(args.pacing_sec)
        for f in as_completed(futures):
            f.result()

    if state["fatal"]:
        sys.stderr.write(f"\nFATAL: {state['fatal']}\n")
    sys.stderr.write(
        f"\nDONE: rated={state['n_done']} unk_skipped={state['n_skipped_unk']} "
        f"cost=${state['cost_usd']:.3f} "
        f"(tokens_in={state['tokens_in']} tokens_out={state['tokens_out']})\n"
    )

    # ---- Final report: include EVERY rated row in --out for this model ----

    claude_by_key = {}
    for r in rows:
        cv = claude_verdict_of(r)
        if cv:
            claude_by_key[key_of(r)] = cv

    openai_by_key = {}
    with open(args.out) as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            if r.get("_openai_model") != args.model:
                continue
            if r.get("_prompt_version") != args.prompt_version:
                continue
            v = (r.get("_openai_verdict") or {}).get("verdict")
            if v in ("TP", "FP", "UNCERTAIN"):
                openai_by_key[key_of(r)] = v

    by_det = per_detector_stats(claude_by_key, openai_by_key)
    pair_breakdown = Counter()
    for key, cv in claude_by_key.items():
        ov = openai_by_key.get(key)
        if ov is None:
            continue
        pair_breakdown[(cv, ov)] += 1

    bar = "=" * 70
    print(f"\n{bar}")
    print(f"FINAL kappa (model={args.model}, prompt={args.prompt_version})")
    print(bar)
    print(f"{'detector':30s} {'n':>4s} {'k3':>6s} {'k2':>6s} {'n2':>4s} {'agree':>6s}  verdict")
    for det in sorted(by_det):
        c, o = by_det[det]
        k3 = cohens_kappa(c, o)
        k2, n2 = kappa_2class(c, o)
        agree = sum(1 for x, y in zip(c, o) if x == y) / max(1, len(c))
        if n2 < 10:
            verdict_str = "n<10 — insufficient"
        elif k2 >= 0.6:
            verdict_str = "OK"
        elif k2 >= 0.4:
            verdict_str = "MARGINAL"
        else:
            verdict_str = "LOW — deprioritize"
        print(f"{det:30s} {len(c):4d} {k3:6.3f} {k2:6.3f} {n2:4d} {100*agree:5.1f}%  {verdict_str}")

    print(f"\n{bar}")
    print("Pair breakdown (Claude verdict, OpenAI verdict)")
    print(bar)
    for (cv, ov), count in sorted(pair_breakdown.items(), key=lambda x: -x[1]):
        marker = "OK " if cv == ov else "!! "
        print(f"  {marker} Claude={cv:10s} OpenAI={ov:10s} {count}")

    pair = lambda c, o: pair_breakdown.get((c, o), 0)
    claude_tp = pair("TP", "TP") + pair("TP", "FP") + pair("TP", "UNCERTAIN")
    claude_fp = pair("FP", "FP") + pair("FP", "TP") + pair("FP", "UNCERTAIN")
    claude_unc = pair("UNCERTAIN", "TP") + pair("UNCERTAIN", "FP") + pair("UNCERTAIN", "UNCERTAIN")

    print(f"\n{bar}")
    print("Asymmetric reliability")
    print(bar)
    if claude_tp:
        print(f"  Claude TP -> OpenAI confirms TP: "
              f"{pair('TP','TP')}/{claude_tp} = {100*pair('TP','TP')/claude_tp:.1f}%")
    if claude_fp:
        print(f"  Claude FP -> OpenAI confirms FP: "
              f"{pair('FP','FP')}/{claude_fp} = {100*pair('FP','FP')/claude_fp:.1f}%")
    if claude_unc:
        print(f"  Claude UNCERTAIN -> OpenAI commits TP: "
              f"{pair('UNCERTAIN','TP')}/{claude_unc} = "
              f"{100*pair('UNCERTAIN','TP')/claude_unc:.1f}% (salvageable Claude punts)")
        print(f"  Claude UNCERTAIN -> OpenAI commits FP: "
              f"{pair('UNCERTAIN','FP')}/{claude_unc} = "
              f"{100*pair('UNCERTAIN','FP')/claude_unc:.1f}%")

    if state["fatal"]:
        sys.exit(2)


if __name__ == "__main__":
    main()
