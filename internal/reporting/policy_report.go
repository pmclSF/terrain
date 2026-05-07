package reporting

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pmclSF/terrain/internal/governance"
	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/uitokens"
)

// RenderPolicyReport writes a human-readable policy check report to w.
//
// Layout (0.2 redesign — audit lift on policy_governance.V2):
//
//	──────────────────────────────────────────────────────────────
//	  [PASS|BLOCKED]  N violations against <policy-file>
//	──────────────────────────────────────────────────────────────
//
//	Policy file: .terrain/policy.yaml
//
//	Violations by severity
//	──────────────────────
//	  [CRIT] safetyFailure (AI) — <explanation>
//	         location: src/agents/run.go
//	  [HIGH] coverageThresholdBreak (Quality) — <explanation>
//	  ...
//
// Group-by-severity and per-violation severity badges replace the
// previous flat "  - <type>: <explanation>" rendering. Adopters can
// scan the most-severe blockers first; the hero block gives the
// overall verdict its own visual weight.
func RenderPolicyReport(w io.Writer, policyPath string, result *governance.Result) {
	line, blank := reportHelpers(w)

	// Hero verdict block — PASS / BLOCKED with violation count.
	verdict, headline := policyHeroLines(policyPath, result)
	fmt.Fprintln(w, uitokens.HeroVerdict(verdict, headline))
	blank()

	// Policy file pointer.
	if policyPath != "" {
		line("Policy file: %s", policyPath)
	} else {
		line("Policy file: (none)")
	}
	blank()

	if len(result.Violations) == 0 {
		// Hero block already says PASS; nothing more to render.
		return
	}

	// Group violations by severity (critical → low). Within a
	// severity, sort by category then type for deterministic output.
	groups := groupViolationsBySeverity(result.Violations)
	line("Violations by severity")
	line(strings.Repeat("─", 40))
	for _, sev := range severityRenderOrder {
		vs := groups[sev]
		if len(vs) == 0 {
			continue
		}
		badge := uitokens.BracketedSeverity(string(sev))
		for _, v := range vs {
			loc := v.Location.File
			if loc == "" {
				loc = v.Location.Repository
			}
			category := string(v.Category)
			if category == "" {
				category = "—"
			}
			line("  %s %s (%s) — %s", badge, v.Type, category, v.Explanation)
			if loc != "" {
				line("         location: %s", loc)
			}
		}
	}
	blank()
}

// severityRenderOrder is the canonical critical-first ordering used
// by the policy report and any other renderer that groups by
// severity.
var severityRenderOrder = []models.SignalSeverity{
	models.SeverityCritical,
	models.SeverityHigh,
	models.SeverityMedium,
	models.SeverityLow,
	models.SeverityInfo,
}

// groupViolationsBySeverity buckets violations into a stable
// severity → []Signal map. Within each bucket violations are
// sorted by Category then Type so ordering is deterministic.
func groupViolationsBySeverity(violations []models.Signal) map[models.SignalSeverity][]models.Signal {
	out := make(map[models.SignalSeverity][]models.Signal, len(severityRenderOrder))
	for _, v := range violations {
		sev := v.Severity
		if sev == "" {
			sev = models.SeverityInfo
		}
		out[sev] = append(out[sev], v)
	}
	for _, vs := range out {
		sort.SliceStable(vs, func(i, j int) bool {
			if vs[i].Category != vs[j].Category {
				return vs[i].Category < vs[j].Category
			}
			return string(vs[i].Type) < string(vs[j].Type)
		})
	}
	return out
}

// policyHeroLines maps the policy result to the (verdict, headline)
// pair the hero block renders. The headline names the violation
// count so a glancing reader knows the scale.
func policyHeroLines(policyPath string, result *governance.Result) (verdict, headline string) {
	switch {
	case policyPath == "":
		return "WARN", "no policy file — `terrain init` will scaffold one"
	case result.Pass:
		return "PASS", fmt.Sprintf("policy clear — %s", policyPath)
	default:
		count := len(result.Violations)
		return "BLOCKED", fmt.Sprintf(
			"%d %s against %s",
			count,
			Plural(count, "violation"),
			policyPath,
		)
	}
}
