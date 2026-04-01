package reporting

import (
	"html/template"
	"io"
	"time"

	"github.com/pmclSF/terrain/internal/analyze"
)

// RenderAnalyzeHTML produces a self-contained HTML report from an analyze.Report.
// The output includes inline CSS and minimal JS — no external dependencies.
func RenderAnalyzeHTML(w io.Writer, report *analyze.Report) error {
	data := htmlData{
		Report:    report,
		Generated: time.Now().UTC().Format(time.RFC3339),
	}
	return htmlTmpl.Execute(w, data)
}

type htmlData struct {
	Report    *analyze.Report
	Generated string
}

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"severityColor": func(s string) string {
		switch s {
		case "critical":
			return "#dc2626"
		case "high":
			return "#ea580c"
		case "medium":
			return "#d97706"
		case "low":
			return "#2563eb"
		default:
			return "#6b7280"
		}
	},
	"bandColor": func(b string) string {
		switch b {
		case "High", "high", "well_protected", "strong":
			return "#16a34a"
		case "Medium", "medium", "partially_protected", "moderate":
			return "#d97706"
		case "Low", "low", "weakly_protected", "weak":
			return "#dc2626"
		default:
			return "#6b7280"
		}
	},
}).Parse(htmlTemplate))

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Terrain Analysis — {{.Report.Repository.Name}}</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; line-height: 1.6; color: #1f2937; background: #f9fafb; padding: 2rem; max-width: 1200px; margin: 0 auto; }
  h1 { font-size: 1.5rem; margin-bottom: 0.25rem; }
  h2 { font-size: 1.1rem; margin-top: 2rem; margin-bottom: 0.75rem; padding-bottom: 0.25rem; border-bottom: 2px solid #e5e7eb; }
  .subtitle { color: #6b7280; font-size: 0.85rem; margin-bottom: 1.5rem; }
  .headline { background: #eff6ff; border-left: 4px solid #3b82f6; padding: 0.75rem 1rem; margin-bottom: 1.5rem; font-size: 0.95rem; border-radius: 0 4px 4px 0; }

  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 1rem; margin-bottom: 1.5rem; }
  .card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 1rem; text-align: center; }
  .card .value { font-size: 1.8rem; font-weight: 700; }
  .card .label { font-size: 0.75rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em; }

  .finding { background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 0.75rem 1rem; margin-bottom: 0.5rem; display: flex; align-items: flex-start; gap: 0.75rem; }
  .badge { display: inline-block; padding: 0.1rem 0.5rem; border-radius: 9999px; font-size: 0.7rem; font-weight: 600; color: #fff; text-transform: uppercase; flex-shrink: 0; }

  table { width: 100%; border-collapse: collapse; margin-bottom: 1rem; background: #fff; border-radius: 6px; overflow: hidden; border: 1px solid #e5e7eb; }
  th { background: #f3f4f6; text-align: left; padding: 0.5rem 0.75rem; font-size: 0.75rem; text-transform: uppercase; color: #6b7280; }
  td { padding: 0.5rem 0.75rem; border-top: 1px solid #f3f4f6; font-size: 0.85rem; }

  .bar-container { display: flex; height: 8px; border-radius: 4px; overflow: hidden; margin: 0.5rem 0; }
  .bar-segment { height: 100%; }

  .dim-row { display: flex; align-items: center; gap: 0.75rem; padding: 0.4rem 0; }
  .dim-label { min-width: 120px; font-size: 0.85rem; }
  .dim-band { padding: 0.1rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; color: #fff; }

  .collapsible { cursor: pointer; user-select: none; }
  .collapsible::before { content: "▶ "; font-size: 0.7rem; }
  .collapsible.open::before { content: "▼ "; }
  .collapsible-content { display: none; }
  .collapsible-content.open { display: block; }

  .footer { margin-top: 2rem; padding-top: 1rem; border-top: 1px solid #e5e7eb; color: #9ca3af; font-size: 0.75rem; }

  @media print {
    body { padding: 0; background: #fff; }
    .collapsible-content { display: block !important; }
  }
</style>
</head>
<body>

<h1>Terrain Analysis</h1>
<div class="subtitle">{{.Report.Repository.Name}} · Generated {{.Generated}}</div>

{{if .Report.Headline}}
<div class="headline">{{.Report.Headline}}</div>
{{end}}

<div class="cards">
  <div class="card"><div class="value">{{.Report.TestsDetected.TestFileCount}}</div><div class="label">Test Files</div></div>
  <div class="card"><div class="value">{{.Report.TestsDetected.TestCaseCount}}</div><div class="label">Test Cases</div></div>
  <div class="card"><div class="value">{{.Report.TestsDetected.CodeUnitCount}}</div><div class="label">Code Units</div></div>
  <div class="card"><div class="value">{{.Report.SignalSummary.Total}}</div><div class="label">Signals</div></div>
  <div class="card"><div class="value">{{len .Report.TestsDetected.Frameworks}}</div><div class="label">Frameworks</div></div>
</div>

{{if .Report.KeyFindings}}
<h2>Key Findings</h2>
{{range .Report.KeyFindings}}
<div class="finding">
  <span class="badge" style="background:{{severityColor .Severity}}">{{.Severity}}</span>
  <div>
    <strong>{{.Title}}</strong>
    {{if .Metric}}<div style="font-size:0.85rem;color:#4b5563;margin-top:0.25rem">{{.Metric}}</div>{{end}}
  </div>
</div>
{{end}}
{{if gt .Report.TotalFindingCount (len .Report.KeyFindings)}}
<div style="font-size:0.85rem;color:#6b7280;margin-top:0.5rem">
  {{.Report.TotalFindingCount}} total findings — run <code>terrain insights</code> for the full list.
</div>
{{end}}
{{end}}

{{if .Report.SignalSummary.Total}}
<h2>Signal Summary</h2>
<table>
  <tr><th>Severity</th><th>Count</th></tr>
  {{if .Report.SignalSummary.Critical}}<tr><td><span class="badge" style="background:#dc2626">critical</span></td><td>{{.Report.SignalSummary.Critical}}</td></tr>{{end}}
  {{if .Report.SignalSummary.High}}<tr><td><span class="badge" style="background:#ea580c">high</span></td><td>{{.Report.SignalSummary.High}}</td></tr>{{end}}
  {{if .Report.SignalSummary.Medium}}<tr><td><span class="badge" style="background:#d97706">medium</span></td><td>{{.Report.SignalSummary.Medium}}</td></tr>{{end}}
  {{if .Report.SignalSummary.Low}}<tr><td><span class="badge" style="background:#2563eb">low</span></td><td>{{.Report.SignalSummary.Low}}</td></tr>{{end}}
</table>
{{end}}

{{if .Report.RiskPosture}}
<h2>Risk Posture</h2>
{{range .Report.RiskPosture}}
<div class="dim-row">
  <span class="dim-label">{{.Dimension}}</span>
  <span class="dim-band" style="background:{{bandColor .Band}}">{{.Band}}</span>
</div>
{{end}}
{{end}}

{{if .Report.CoverageConfidence.TotalFiles}}
<h2>Coverage Confidence</h2>
<div class="bar-container">
  {{if .Report.CoverageConfidence.HighCount}}<div class="bar-segment" style="background:#16a34a;flex:{{.Report.CoverageConfidence.HighCount}}"></div>{{end}}
  {{if .Report.CoverageConfidence.MediumCount}}<div class="bar-segment" style="background:#d97706;flex:{{.Report.CoverageConfidence.MediumCount}}"></div>{{end}}
  {{if .Report.CoverageConfidence.LowCount}}<div class="bar-segment" style="background:#dc2626;flex:{{.Report.CoverageConfidence.LowCount}}"></div>{{end}}
</div>
<div style="font-size:0.85rem;color:#6b7280">
  {{.Report.CoverageConfidence.HighCount}} high · {{.Report.CoverageConfidence.MediumCount}} medium · {{.Report.CoverageConfidence.LowCount}} low · {{.Report.CoverageConfidence.TotalFiles}} total files
</div>
{{end}}

{{if .Report.RepoProfile.TestVolume}}
<h2>Repository Profile</h2>
<table>
  <tr><th>Dimension</th><th>Band</th></tr>
  <tr><td>Test Volume</td><td>{{.Report.RepoProfile.TestVolume}}</td></tr>
  <tr><td>CI Pressure</td><td>{{.Report.RepoProfile.CIPressure}}</td></tr>
  <tr><td>Coverage Confidence</td><td>{{.Report.RepoProfile.CoverageConfidence}}</td></tr>
  <tr><td>Redundancy Level</td><td>{{.Report.RepoProfile.RedundancyLevel}}</td></tr>
  <tr><td>Fanout Burden</td><td>{{.Report.RepoProfile.FanoutBurden}}</td></tr>
</table>
{{end}}

{{if .Report.DataCompleteness}}
<h2>Data Completeness</h2>
<table>
  <tr><th>Source</th><th>Status</th></tr>
  {{range .Report.DataCompleteness}}
  <tr><td>{{.Name}}</td><td>{{if .Available}}available{{else}}unavailable{{end}}</td></tr>
  {{end}}
</table>
{{end}}

{{if .Report.Limitations}}
<h2>Limitations</h2>
<ul style="padding-left:1.5rem;font-size:0.85rem;color:#6b7280">
{{range .Report.Limitations}}
  <li>{{.}}</li>
{{end}}
</ul>
{{end}}

<div class="footer">
  Generated by <a href="https://github.com/pmclSF/terrain">Terrain</a> · {{.Generated}}
</div>

<script>
document.querySelectorAll('.collapsible').forEach(el => {
  el.addEventListener('click', () => {
    el.classList.toggle('open');
    el.nextElementSibling.classList.toggle('open');
  });
});
</script>
</body>
</html>`
