#!/usr/bin/env bash
# Discover AI/ML projects on GitLab and classify each as:
#   - "gitlab-only"           : no matching GitHub repo found
#   - "mirror-from-github"    : GitLab's `mirror` flag is true, points to GitHub
#   - "also-on-github"        : same owner/repo exists on GitHub (likely cross-published)
#   - "unknown"               : can't determine
#
# Output: tier-4/gitlab-ai-coverage.jsonl  (one record per GitLab project)
#
# Auth: GitLab public API is unauthenticated for read (rate ~60/min/IP).
# GitHub equivalence check requires GITHUB_TOKEN (5000/hour authed vs 60/hour
# unauthed) — without it we skip the cross-platform check.
#
# Topics queried:  ai, machine-learning, llm, deep-learning, pytorch,
#                  tensorflow, transformers, langchain
#
# Usage:
#   GITHUB_TOKEN=ghp_... bash scripts/discover-gitlab-ai.sh
#   PER_TOPIC_PAGES=2 bash scripts/discover-gitlab-ai.sh   (faster smoke test)

set -uo pipefail
cd "$(dirname "$0")/.."

OUT="${OUT:-tier-4/gitlab-ai-coverage.jsonl}"
PER_TOPIC_PAGES="${PER_TOPIC_PAGES:-5}"   # GitLab caps at 100/page
PER_PAGE=100
GITHUB_CHECK="${GITHUB_CHECK:-1}"          # set to 0 to skip GH lookups

if [[ -z "${GITHUB_TOKEN:-}" ]] && [[ "$GITHUB_CHECK" == "1" ]]; then
  echo "[gitlab-discover] WARNING: GITHUB_TOKEN not set — disabling GitHub equivalence check" >&2
  GITHUB_CHECK=0
fi

log() { echo "[$(date +%H:%M:%S)] [gitlab-discover] $*" >&2; }

TOPICS=(
  "machine-learning" "deep-learning" "pytorch" "tensorflow"
  "llm" "transformers" "langchain" "openai" "huggingface"
  "neural-network" "ai" "artificial-intelligence"
)

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

for topic in "${TOPICS[@]}"; do
  log "topic: $topic"
  for ((page=1; page<=PER_TOPIC_PAGES; page++)); do
    sleep 1
    resp=$(curl -fsS \
      "https://gitlab.com/api/v4/projects?topic=${topic}&per_page=${PER_PAGE}&page=${page}&order_by=star_count&sort=desc" 2>/dev/null) || continue
    n=$(echo "$resp" | python3 -c 'import sys,json;d=json.load(sys.stdin);print(len(d))' 2>/dev/null) || break
    if [[ "$n" -eq 0 ]]; then break; fi
    echo "$resp" | python3 -c "
import sys, json
for item in json.load(sys.stdin):
    rec = {
        'platform': 'gitlab',
        'gitlab_id': item.get('id'),
        'path_with_namespace': item.get('path_with_namespace'),
        'web_url': item.get('web_url'),
        'description': item.get('description') or '',
        'star_count': item.get('star_count', 0),
        'forks_count': item.get('forks_count', 0),
        'last_activity_at': item.get('last_activity_at'),
        'default_branch': item.get('default_branch'),
        'topics': item.get('topics', []),
        'mirror': item.get('mirror', False),
        'topic_discovered_via': '$topic',
    }
    print(json.dumps(rec))
" >> "$TMP"
  done
done

# Dedupe by path_with_namespace, keep highest star count.
python3 -c "
import json
best = {}
for line in open('$TMP'):
    try: r = json.loads(line)
    except: continue
    key = r['path_with_namespace']
    if key and (key not in best or r['star_count'] > best[key]['star_count']):
        best[key] = r
print(f'[gitlab-discover] {len(best)} unique GitLab projects after dedupe', file=__import__('sys').stderr)
for r in sorted(best.values(), key=lambda x: -x['star_count']):
    print(json.dumps(r))
" > "$TMP.dedup"

if [[ "$GITHUB_CHECK" == "1" ]]; then
  log "Cross-checking against GitHub for $(wc -l < "$TMP.dedup" | tr -d ' ') projects"
  python3 - "$TMP.dedup" "$OUT" <<'PYEOF'
import json, os, sys, urllib.request, urllib.error, time

in_path, out_path = sys.argv[1], sys.argv[2]
token = os.environ.get('GITHUB_TOKEN', '')
headers = {'Authorization': f'Bearer {token}'} if token else {}
headers['Accept'] = 'application/vnd.github+json'

checked = 0
out = open(out_path, 'w')
for line in open(in_path):
    rec = json.loads(line)
    path = rec['path_with_namespace']
    # Strip any sub-group nesting — only consider top-level owner/repo
    # for GitHub equivalence.
    parts = path.split('/')
    if len(parts) < 2:
        rec['classification'] = 'unknown'
        rec['github_equivalent'] = None
        out.write(json.dumps(rec) + '\n')
        continue
    # GitLab allows subgroups; GitHub doesn't. Use the LAST segment
    # as the repo name and the FIRST as the owner, mirroring how
    # most cross-published projects appear.
    owner, repo = parts[0], parts[-1]
    url = f'https://api.github.com/repos/{owner}/{repo}'
    req = urllib.request.Request(url, headers=headers)
    gh_exists = None
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            gh = json.loads(resp.read())
            gh_exists = f"{gh['full_name']}"
    except urllib.error.HTTPError as e:
        if e.code == 404:
            gh_exists = None
        elif e.code in (403, 429):
            time.sleep(60)  # rate limit cooldown
            continue
        else:
            gh_exists = '__error__'
    except Exception:
        gh_exists = '__error__'

    if rec.get('mirror'):
        cls = 'mirror-from-elsewhere'
    elif gh_exists is None:
        cls = 'gitlab-only'
    elif gh_exists == '__error__':
        cls = 'unknown'
    else:
        cls = 'also-on-github'

    rec['github_equivalent'] = gh_exists if gh_exists != '__error__' else None
    rec['classification'] = cls
    out.write(json.dumps(rec) + '\n')
    checked += 1
    if checked % 100 == 0:
        print(f'[gitlab-discover] checked {checked} (last: {path} → {cls})',
              file=sys.stderr)
    # GitHub authed rate is 5000/hour = ~83/min. Sleep to stay well under.
    time.sleep(0.8)
out.close()
print(f'[gitlab-discover] cross-check done — {checked} projects classified',
      file=sys.stderr)
PYEOF
else
  cp "$TMP.dedup" "$OUT"
  log "skipped GitHub cross-check; output is GitLab-only metadata"
fi

n=$(wc -l < "$OUT" | tr -d ' ')
log "wrote $n records to $OUT"

# Print summary breakdown
if grep -q classification "$OUT" 2>/dev/null; then
  echo
  echo "=== Classification breakdown ==="
  python3 -c "
import json, collections
counts = collections.Counter()
stars = collections.defaultdict(int)
for line in open('$OUT'):
    r = json.loads(line)
    cls = r.get('classification', 'no-check')
    counts[cls] += 1
    stars[cls] += r.get('star_count', 0)
total = sum(counts.values())
for cls, n in sorted(counts.items(), key=lambda x: -x[1]):
    pct = 100 * n / total if total else 0
    avg_stars = stars[cls] / n if n else 0
    print(f'  {cls:25} {n:5} ({pct:5.1f}%)  avg stars: {avg_stars:.0f}')
print(f'  {\"TOTAL\":25} {total:5}')
"
fi
