package explain

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FindingLineage answers "how old is this finding?" by running git
// blame against the underlying line. The answer is purely informational
// — Terrain doesn't use it for gating — but it changes how an adopter
// reads a finding. "Untested export added three days ago" is a code-
// review oversight; "untested export from 2022" is technical debt the
// team has lived with. The triage decision differs.
//
// Returns nil silently if git blame fails (not a repo, file not
// tracked, line doesn't exist in the current revision). Callers
// should treat missing lineage as "no information" rather than an
// error to surface.
type FindingLineage struct {
	CommitSHA    string    // 40-char SHA introducing the line
	ShortSHA     string    // 8-char prefix for display
	Author       string    // committer name
	AuthorEmail  string    // committer email
	AuthoredAt   time.Time // commit timestamp
	CommitsSince int       // number of commits to this file since AuthoredAt (HEAD-relative)
}

// LookupLineage runs `git blame` against (file, line) and parses the
// porcelain output. The repoRoot argument should be the working tree
// root; file is interpreted relative to it.
func LookupLineage(repoRoot, file string, line int) (*FindingLineage, error) {
	if file == "" || line <= 0 {
		return nil, nil
	}
	out, err := exec.Command("git", "-C", repoRoot, "blame",
		"-L", fmt.Sprintf("%d,%d", line, line),
		"--porcelain", "--", file).Output()
	if err != nil {
		return nil, nil // not a repo, file untracked — silently no-op
	}

	l := &FindingLineage{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		ln := sc.Text()
		switch {
		case l.CommitSHA == "" && len(ln) >= 40 && isHex(ln[:40]):
			l.CommitSHA = ln[:40]
			if len(l.CommitSHA) >= 8 {
				l.ShortSHA = l.CommitSHA[:8]
			}
		case strings.HasPrefix(ln, "author "):
			l.Author = strings.TrimPrefix(ln, "author ")
		case strings.HasPrefix(ln, "author-mail "):
			l.AuthorEmail = strings.Trim(strings.TrimPrefix(ln, "author-mail "), "<>")
		case strings.HasPrefix(ln, "author-time "):
			if ts, err := strconv.ParseInt(strings.TrimPrefix(ln, "author-time "), 10, 64); err == nil {
				l.AuthoredAt = time.Unix(ts, 0)
			}
		}
	}
	if l.CommitSHA == "" {
		return nil, nil
	}

	// Count file-touching commits between the introducing SHA and HEAD.
	// "How many commits has the file seen since this line was written" —
	// a useful proxy for how much surrounding context has shifted.
	cntOut, cntErr := exec.Command("git", "-C", repoRoot, "rev-list", "--count",
		l.CommitSHA+"..HEAD", "--", file).Output()
	if cntErr == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(cntOut))); err == nil {
			l.CommitsSince = n
		}
	}
	return l, nil
}

func isHex(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// FormatAge produces a human-readable description like "3 days ago"
// or "1 year ago" for the lineage's commit timestamp.
func (l *FindingLineage) FormatAge(now time.Time) string {
	if l == nil || l.AuthoredAt.IsZero() {
		return ""
	}
	d := now.Sub(l.AuthoredAt)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 90*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	case d < 2*365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d years ago", int(d.Hours()/(24*365)))
	}
}
