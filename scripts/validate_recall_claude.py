#!/usr/bin/env python3
"""
Recall validation for terrain's *local* (non-graph) detectors via Claude CLI.

For each sampled source file:
  1. Send file content + per-detector checklist to Claude CLI as an oracle.
  2. Parse Claude's per-detector yes/no + one-line evidence.
  3. Intersect with terrain snapshot.signals for that file.
  4. recall_X = (Claude_yes ∩ terrain_yes) / Claude_yes, per detector.

Local detectors only — file-resolvable, not graph-shaped:
    weakAssertion, untestedExport, mockHeavyTest, uncoveredAISurface,
    assertionFreeTest, staticSkippedTest, orphanedTestFile

Excluded (need separate repo-mode methodology):
    blastRadiusHotspot, fixtureFragilityHotspot — both require import-graph
    context that a single file doesn't carry. File-mode would systematically
    under-flag → terrain would look artificially high-recall.

Usage:
    python3 scripts/validate_recall_claude.py \\
        --repo-list tier-4/recall-repos.txt \\
        --max-repos 50 \\
        --files-per-repo 5 \\
        --output tier-4/detector-recall.jsonl
"""

from __future__ import annotations
import argparse
import json
import random
import subprocess
import sys
import time
from collections import defaultdict
from pathlib import Path

LOCAL_DETECTORS = {
    "weakAssertion":
        "An assertion in a test that checks too little. Examples of WEAK: "
        "`assert x`, `assert x is not None`, `expect(result).toBeTruthy()`, "
        "`expect(x).toBeDefined()`. Examples of STRONG: `assert x == 42`, "
        "`expect(result).toEqual({a: 1, b: 2})`. Only mark YES if you can "
        "point to a specific weak assertion in this file.",
    "untestedExport":
        "A public/exported function, class, or constant defined in this file. "
        "Since you only see one file, mark YES if this file declares public "
        "API (top-level `def`/`class` in Python, `func`/`type` without lowercase "
        "first letter in Go, `export` in TS/JS). The downstream comparison "
        "checks whether tests actually import it.",
    "mockHeavyTest":
        "Only applies if this is a TEST file. Heavy use of mocks/stubs such "
        "that most behaviors are mocked rather than exercised: many "
        "`Mock()`/`patch()` calls in Python, `jest.mock()` in JS/TS, etc., "
        "and few real assertions on real behavior.",
    "uncoveredAISurface":
        "A prompt template, system message, or LLM tool definition declared "
        "in this file. Examples: a multi-line prompt string assigned to a "
        "variable, a `ChatPromptTemplate`, a tool/function-calling schema, "
        "a system prompt. Mark YES if any AI surface is declared here.",
    "assertionFreeTest":
        "Only applies if this is a TEST file. A test function (e.g., "
        "`def test_*`, `it(...)`, `@Test`) that contains ZERO assertions or "
        "expectations — purely setup or print statements.",
    "staticSkippedTest":
        "Only applies if this is a TEST file. A test that is permanently "
        "skipped without a re-enable plan: `@pytest.mark.skip` without a "
        "reason that references a ticket, `it.skip` / `xit`, `@Ignore` "
        "without an issue link or TODO with date.",
    "orphanedTestFile":
        "Only applies if this is a TEST file. The file appears to test a "
        "module that no longer exists (heavy use of imports/symbols that "
        "look stale, e.g., imports a module path that follows the test's "
        "naming convention but you'd expect to see in the project).",
}

PROMPT_TEMPLATE = """You are reviewing one source file for code-quality issues.

File path: {file_path}
Language: {language}

```{language_tag}
{content}
```

For each of the following detector categories, answer YES or NO based ONLY on what is visible in this file. When YES, give a one-line evidence quote from the file.

{checklist}

Return ONLY a JSON object in this exact shape, with no preamble or trailing text:
{{
  "weakAssertion": {{"yes": false, "evidence": ""}},
  "untestedExport": {{"yes": false, "evidence": ""}},
  "mockHeavyTest": {{"yes": false, "evidence": ""}},
  "uncoveredAISurface": {{"yes": false, "evidence": ""}},
  "assertionFreeTest": {{"yes": false, "evidence": ""}},
  "staticSkippedTest": {{"yes": false, "evidence": ""}},
  "orphanedTestFile": {{"yes": false, "evidence": ""}}
}}
"""


def build_checklist() -> str:
    lines = []
    for i, (k, desc) in enumerate(LOCAL_DETECTORS.items(), 1):
        lines.append(f"{i}. **{k}**: {desc}")
    return "\n\n".join(lines)


def detect_language(path: str) -> tuple[str, str]:
    if path.endswith(".py"):
        return "Python", "python"
    if path.endswith(".go"):
        return "Go", "go"
    if path.endswith((".ts", ".tsx")):
        return "TypeScript", "typescript"
    if path.endswith((".js", ".jsx")):
        return "JavaScript", "javascript"
    return "code", ""


def call_claude(prompt: str, timeout: int = 120) -> dict:
    try:
        result = subprocess.run(
            ["claude", "-p", prompt],
            capture_output=True, text=True, timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return {"_error": "timeout"}
    if result.returncode != 0:
        return {"_error": f"exit {result.returncode}",
                "_stderr": result.stderr[:300]}
    raw = result.stdout.strip()
    start = raw.find("{")
    end = raw.rfind("}")
    if start < 0 or end < 0:
        return {"_error": "no JSON in response", "_raw": raw[:500]}
    try:
        return json.loads(raw[start:end + 1])
    except json.JSONDecodeError as e:
        return {"_error": f"json parse: {e}",
                "_raw": raw[start:end + 1][:500]}


def run_terrain(repo_path: Path, terrain_bin: str) -> dict:
    """Run `terrain analyze --write-snapshot` and parse the snapshot.

    `terrain insights --json` only returns aggregate findings; the granular
    per-signal data lives in .terrain/snapshots/latest.json which is only
    written by analyze --write-snapshot.
    """
    try:
        subprocess.run(
            [terrain_bin, "analyze", "--write-snapshot",
             "--root", str(repo_path)],
            capture_output=True, timeout=300,
        )
    except subprocess.TimeoutExpired:
        return {}
    snap_path = repo_path / ".terrain" / "snapshots" / "latest.json"
    if not snap_path.exists():
        return {}
    try:
        return json.loads(snap_path.read_text())
    except Exception:
        return {}


def signals_by_file(snapshot: dict) -> dict[str, set[str]]:
    out: dict[str, set[str]] = defaultdict(set)
    for s in snapshot.get("signals", []):
        loc = s.get("location") or {}
        f = loc.get("file", "")
        t = s.get("type", "")
        if f and t:
            out[f].add(t)
    return out


def find_source_files(repo_path: Path, exts: tuple[str, ...],
                      cap: int = 300) -> list[Path]:
    files = []
    for p in repo_path.rglob("*"):
        if not p.is_file():
            continue
        if any(part.startswith(".") for part in p.parts):
            continue
        if "node_modules" in p.parts or "vendor" in p.parts:
            continue
        if p.suffix in exts:
            files.append(p)
            if len(files) >= cap:
                break
    return files


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo-list", required=True,
                    help="one owner__name per line")
    ap.add_argument("--terrain-bin", default="/usr/local/bin/terrain")
    ap.add_argument("--workdir", default="/tmp/recall-validation")
    ap.add_argument("--output", default="tier-4/detector-recall.jsonl")
    ap.add_argument("--max-repos", type=int, default=50)
    ap.add_argument("--files-per-repo", type=int, default=5)
    ap.add_argument("--depth", type=int, default=1)
    ap.add_argument("--seed", type=int, default=42)
    args = ap.parse_args()

    random.seed(args.seed)

    repo_list_path = Path(args.repo_list)
    if not repo_list_path.exists():
        sys.exit(f"repo list not found: {repo_list_path}")
    work_root = Path(args.workdir)
    work_root.mkdir(parents=True, exist_ok=True)
    out_path = Path(args.output)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    repos = []
    for line in repo_list_path.open():
        n = line.strip()
        if n and not n.startswith("#"):
            repos.append(n)
    random.shuffle(repos)
    repos = repos[: args.max_repos]
    sys.stderr.write(
        f"[recall] {len(repos)} repos, {args.files_per_repo} files/repo "
        f"(~{len(repos) * args.files_per_repo} files total)\n"
    )

    checklist = build_checklist()
    exts = (".py", ".go", ".ts", ".tsx", ".js", ".jsx")

    out_f = out_path.open("w")
    start_t = time.time()
    n_files = 0
    for ri, repo in enumerate(repos, 1):
        parts = repo.split("__", 1)
        if len(parts) != 2:
            continue
        owner, name = parts
        url = f"https://github.com/{owner}/{name}.git"
        target = work_root / repo

        try:
            subprocess.run(
                ["git", "clone", f"--depth={args.depth}",
                 "--filter=blob:limit=512k", "--no-tags", "-q",
                 url, str(target)],
                check=True, timeout=300, capture_output=True,
            )
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
            sys.stderr.write(f"[recall] FAIL clone {repo}\n")
            continue

        snap = run_terrain(target, args.terrain_bin)
        if not snap:
            sys.stderr.write(f"[recall] FAIL terrain {repo}\n")
            subprocess.run(["rm", "-rf", str(target)], check=False)
            continue
        sigs_map = signals_by_file(snap)

        files = find_source_files(target, exts)
        if not files:
            subprocess.run(["rm", "-rf", str(target)], check=False)
            continue
        random.shuffle(files)
        chosen = files[: args.files_per_repo]

        for fp in chosen:
            try:
                content = fp.read_text(errors="replace")
            except Exception:
                continue
            if len(content) > 8000:
                content = content[:8000] + "\n... [truncated]"
            rel = fp.relative_to(target).as_posix()
            lang, lang_tag = detect_language(rel)
            prompt = PROMPT_TEMPLATE.format(
                file_path=rel, language=lang, language_tag=lang_tag,
                content=content, checklist=checklist,
            )
            verdicts = call_claude(prompt)
            row = {
                "repo": repo,
                "file": rel,
                "language": lang,
                "claude_verdicts": verdicts,
                "terrain_signals": sorted(sigs_map.get(rel, set())),
                "ts": int(time.time()),
            }
            out_f.write(json.dumps(row) + "\n")
            out_f.flush()
            n_files += 1
            if n_files % 5 == 0:
                rate = n_files / max(1, time.time() - start_t)
                sys.stderr.write(
                    f"[recall] {n_files} files rated @ {rate:.1f}/s "
                    f"(repo {ri}/{len(repos)})\n"
                )

        subprocess.run(["rm", "-rf", str(target)], check=False)

    out_f.close()
    elapsed = time.time() - start_t
    sys.stderr.write(
        f"[recall] DONE: {n_files} files rated in {elapsed/60:.1f} min\n"
    )


if __name__ == "__main__":
    main()
