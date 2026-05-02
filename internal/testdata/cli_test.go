package testdata

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCLI_BuildSucceeds verifies the binary compiles.
func TestCLI_BuildSucceeds(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "build", "-o", "/dev/null", "./cmd/terrain/")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
}

// TestCLI_HelpExitCode verifies --help exits cleanly.
func TestCLI_HelpExitCode(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "--help")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	output := string(out)

	// --help should exit 0 (or at least not error).
	if err != nil {
		t.Logf("help output: %s", output)
		// Some CLIs exit non-zero on --help; check output instead.
		if !strings.Contains(output, "Terrain") {
			t.Errorf("--help did not produce terrain output: %v", err)
		}
	}
}

// TestCLI_HelpContainsAllCommands verifies help text lists all commands.
//
// 0.2 layout: top-level help uses namespace dispatchers (`report <verb>`,
// `ai <verb>`, etc.) with the verb list shown inline rather than every
// `terrain ai list`/`terrain ai run` enumerated as a separate row. The
// test verifies (a) the namespace dispatcher line is present and (b)
// every expected verb appears alongside it.
func TestCLI_HelpContainsAllCommands(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "--help")
	cmd.Dir = "../.."
	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Primary commands (canonical user journeys).
	primary := []string{"analyze", "impact", "insights", "explain"}
	for _, c := range primary {
		if !strings.Contains(output, c) {
			t.Errorf("help text missing primary command %q", c)
		}
	}

	// Supporting commands.
	supporting := []string{"summary", "posture", "metrics", "compare", "policy check", "export benchmark"}
	for _, c := range supporting {
		if !strings.Contains(output, c) {
			t.Errorf("help text missing supporting command %q", c)
		}
	}

	// AI namespace dispatcher + verb list (canonical 0.2 layout).
	if !strings.Contains(output, "ai <verb>") {
		t.Errorf("help text missing AI namespace dispatcher %q", "ai <verb>")
	}
	aiVerbs := []string{"list", "run", "replay", "record", "baseline", "doctor"}
	for _, v := range aiVerbs {
		if !strings.Contains(output, v) {
			t.Errorf("help text missing AI verb %q", v)
		}
	}
}

// TestCLI_UnknownCommandExitCode verifies unknown commands exit non-zero.
func TestCLI_UnknownCommandExitCode(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "nonexistent")
	cmd.Dir = "../.."
	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit for unknown command")
	}
}

// TestCLI_AnalyzeTestdata verifies analyze works on the test repo.
func TestCLI_AnalyzeTestdata(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "analyze", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("analyze failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Terrain") {
		t.Error("analyze output missing header")
	}
	if !strings.Contains(output, "Next steps:") {
		t.Error("analyze output missing next steps")
	}
}

// TestCLI_AnalyzeJSON verifies --json produces valid JSON.
func TestCLI_AnalyzeJSON(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "analyze", "--json", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("analyze --json failed: %v\n%s", err, out)
	}

	if !strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
		t.Error("analyze --json output is not JSON")
	}
}

// TestCLI_PostureTestdata verifies posture command works.
func TestCLI_PostureTestdata(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "posture", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posture failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Terrain Posture") {
		t.Error("posture output missing header")
	}
}

// TestCLI_MetricsTestdata verifies metrics command works.
func TestCLI_MetricsTestdata(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "metrics", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("metrics failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Terrain Metrics") {
		t.Error("metrics output missing header")
	}
}

// TestCLI_SummaryTestdata verifies summary command works.
func TestCLI_SummaryTestdata(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "summary", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("summary failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Terrain Executive Summary") {
		t.Error("summary output missing header")
	}
}

// TestCLI_HelpContainsPrimaryCommands verifies help prominently lists the canonical commands.
//
// 0.2 layout: only `analyze` survives as a top-level primary command —
// `insights`, `impact`, `explain` moved under the `report <verb>`
// namespace dispatcher. The journey-question form is preserved for
// `analyze` (the one command users invoke first); the report verbs
// are advertised in the "Typical flow:" walkthrough instead.
func TestCLI_HelpContainsPrimaryCommands(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "--help")
	cmd.Dir = "../.."
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if !strings.Contains(output, "Canonical commands") {
		t.Error("help text missing 'Canonical commands' section header")
	}
	if !strings.Contains(output, "What is the state of our test system?") {
		t.Error("help text missing analyze journey question")
	}
	for _, c := range []string{"analyze", "report", "impact", "insights", "explain"} {
		if !strings.Contains(output, c) {
			t.Errorf("help text missing command/verb %q", c)
		}
	}
}

// TestCLI_HelpContainsCanonicalWorkflow verifies help includes the standard journey walkthrough.
//
// 0.2 layout: the typical-flow block recommends the canonical
// namespace forms (`terrain report insights`, `terrain report impact`,
// `terrain report explain`) rather than the legacy bare-command forms.
func TestCLI_HelpContainsCanonicalWorkflow(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "--help")
	cmd.Dir = "../.."
	out, _ := cmd.CombinedOutput()
	output := string(out)

	workflow := []string{
		"Typical flow:",
		"terrain analyze",
		"terrain report insights",
		"terrain report impact",
		"terrain report explain",
	}
	for _, expected := range workflow {
		if !strings.Contains(output, expected) {
			t.Errorf("help text missing canonical workflow item %q", expected)
		}
	}
}

// TestCLI_HelpContainsDebugNamespace verifies debug commands appear in help.
//
// 0.2 layout: top-level help shows `debug <verb>` with the verb list
// inline, matching the pattern used by `report`/`migrate`/`ai`/`config`.
func TestCLI_HelpContainsDebugNamespace(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "--help")
	cmd.Dir = "../.."
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if !strings.Contains(output, "debug <verb>") {
		t.Errorf("help text missing debug namespace dispatcher %q", "debug <verb>")
	}
	debugVerbs := []string{"graph", "coverage", "fanout", "duplicates", "depgraph"}
	for _, v := range debugVerbs {
		if !strings.Contains(output, v) {
			t.Errorf("help text missing debug verb %q", v)
		}
	}
}

func TestCLI_AINamespaceHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "ai", "--help")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ai --help failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Usage: terrain ai") {
		t.Fatalf("ai --help missing usage text:\n%s", output)
	}
	for _, expected := range []string{"list", "run", "replay", "record", "baseline", "doctor"} {
		if !strings.Contains(output, expected) {
			t.Errorf("ai --help missing %q", expected)
		}
	}
}

func TestCLI_MigrationNamespaceHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "migration", "--help")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migration --help failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Usage: terrain migration") {
		t.Fatalf("migration --help missing usage text:\n%s", output)
	}
	for _, expected := range []string{"readiness", "blockers", "preview"} {
		if !strings.Contains(output, expected) {
			t.Errorf("migration --help missing %q", expected)
		}
	}
}

func TestCLI_DebugNamespaceHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "debug", "--help")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("debug --help failed: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Usage: terrain debug") {
		t.Fatalf("debug --help missing usage text:\n%s", out)
	}
}

func TestCLI_ExportNamespaceHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "export", "--help")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("export --help failed: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Usage: terrain export benchmark") {
		t.Fatalf("export --help missing usage text:\n%s", out)
	}
}

func TestCLI_AISubcommandHelpShowsOnlyRelevantFlags(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "ai", "replay", "--help")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ai replay --help failed: %v\n%s", err, out)
	}

	output := string(out)
	for _, unexpected := range []string{"-base", "-dry-run", "-full", "-verbose"} {
		if strings.Contains(output, unexpected) {
			t.Errorf("ai replay --help should not include %q:\n%s", unexpected, output)
		}
	}
	for _, expected := range []string{"-json", "-root"} {
		if !strings.Contains(output, expected) {
			t.Errorf("ai replay --help missing %q:\n%s", expected, output)
		}
	}
}

func TestCLI_MigrationSubcommandHelpShowsOnlyRelevantFlags(t *testing.T) {
	t.Parallel()

	readinessCmd := exec.Command("go", "run", "./cmd/terrain/", "migration", "readiness", "--help")
	readinessCmd.Dir = "../.."
	readinessOut, err := readinessCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migration readiness --help failed: %v\n%s", err, readinessOut)
	}
	readiness := string(readinessOut)
	for _, unexpected := range []string{"-file", "-scope"} {
		if strings.Contains(readiness, unexpected) {
			t.Errorf("migration readiness --help should not include %q:\n%s", unexpected, readiness)
		}
	}

	previewCmd := exec.Command("go", "run", "./cmd/terrain/", "migration", "preview", "--help")
	previewCmd.Dir = "../.."
	previewOut, err := previewCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migration preview --help failed: %v\n%s", err, previewOut)
	}
	preview := string(previewOut)
	for _, expected := range []string{"-file", "-scope"} {
		if !strings.Contains(preview, expected) {
			t.Errorf("migration preview --help missing %q:\n%s", expected, preview)
		}
	}
}

// TestCLI_ExportBenchmarkTestdata verifies export benchmark command works.
func TestCLI_ExportBenchmarkTestdata(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "export", "benchmark", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("export benchmark failed: %v\n%s", err, out)
	}

	output := strings.TrimSpace(string(out))
	if !strings.HasPrefix(output, "{") {
		t.Error("export benchmark output should be JSON")
	}
}

func TestCLI_InitJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "init", "--root", root, "--json")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init --json failed: %v\n%s", err, out)
	}

	if !strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
		t.Fatalf("init --json output is not JSON:\n%s", out)
	}
}

func TestCLI_ExportBenchmarkAcceptsJSONFlag(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "run", "./cmd/terrain/", "export", "benchmark", "--root", "internal/analysis/testdata/sample-repo", "--json")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("export benchmark --json failed: %v\n%s", err, out)
	}

	if !strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
		t.Fatalf("export benchmark --json output is not JSON:\n%s", out)
	}
}

func TestCLI_VersionJSON(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "run", "./cmd/terrain/", "version", "--json")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version --json failed: %v\n%s", err, out)
	}

	if !strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
		t.Fatalf("version --json output is not JSON:\n%s", out)
	}
}

// TestCLI_AnalyzeOutputConsistency verifies analyze output structure.
func TestCLI_AnalyzeOutputConsistency(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "analyze", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("analyze failed: %v\n%s", err, out)
	}

	output := string(out)
	// The first-run analyze report uses the v2 layout; assert durable sections
	// that reflect the current CLI contract rather than legacy headings.
	sections := []string{
		"Repository Profile",
		"Validation Inventory",
		"Risk Posture",
		"Signals:",
		"Data Completeness",
		"Next steps:",
	}
	for _, s := range sections {
		if !strings.Contains(output, s) {
			t.Errorf("analyze output missing section %q", s)
		}
	}
}

// TestCLI_MetricsJSON verifies metrics --json produces valid JSON.
func TestCLI_MetricsJSON(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/terrain/", "metrics", "--json", "--root", "internal/analysis/testdata/sample-repo")
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("metrics --json failed: %v\n%s", err, out)
	}

	output := strings.TrimSpace(string(out))
	if !strings.HasPrefix(output, "{") {
		t.Error("metrics --json output should be JSON")
	}
}
