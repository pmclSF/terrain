package quality

import (
	"strings"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
)

func TestUntestedExportDetector_NoLinkedTests(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "src/__tests__/auth.test.js"},
		},
		CodeUnits: []models.CodeUnit{
			{Name: "fetchData", Path: "src/services/api.js", Kind: models.CodeUnitKindFunction, Exported: true},
		},
	}

	d := &UntestedExportDetector{}
	signals := d.Detect(snap)

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Type != "untestedExport" {
		t.Errorf("type = %q, want untestedExport", signals[0].Type)
	}
	if signals[0].Location.Symbol != "fetchData" {
		t.Errorf("symbol = %q, want fetchData", signals[0].Location.Symbol)
	}
}

func TestUntestedExportDetector_HasNearbyTest(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "src/__tests__/auth.test.js"},
		},
		CodeUnits: []models.CodeUnit{
			{Name: "authenticate", Path: "src/auth.js", Kind: models.CodeUnitKindFunction, Exported: true},
		},
	}

	d := &UntestedExportDetector{}
	signals := d.Detect(snap)

	// "auth" stem matches auth.test.js stem — should not flag
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for code with nearby test, got %d", len(signals))
	}
}

func TestUntestedExportDetector_UnexportedIgnored(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{},
		CodeUnits: []models.CodeUnit{
			{Name: "internalHelper", Path: "src/util.js", Kind: models.CodeUnitKindFunction, Exported: false},
		},
	}

	d := &UntestedExportDetector{}
	signals := d.Detect(snap)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for unexported code unit, got %d", len(signals))
	}
}

func TestUntestedExportDetector_NoCodeUnits(t *testing.T) {
	t.Parallel()
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "test/a.test.js"},
		},
	}

	d := &UntestedExportDetector{}
	signals := d.Detect(snap)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals with no code units, got %d", len(signals))
	}
}

func TestUntestedExportDetector_SkipsToolingPaths(t *testing.T) {
	t.Parallel()
	// Tooling paths under .github/actions/ (deploy/CI scripts) must not
	// be flagged as untested exports; they don't need unit-test coverage.
	snap := &models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{
			{Path: "src/__tests__/auth.test.js"},
		},
		CodeUnits: []models.CodeUnit{
			{Name: "deployToFirebase", Path: ".github/actions/deploy-docs-site/lib/deploy.mts", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "buildBenchmark", Path: "packages/benchpress/scripts/run.ts", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "buildArtifact", Path: "tools/codegen/run.ts", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "binaryEntry", Path: "bin/cli.ts", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "exampleFn", Path: "examples/usage.ts", Kind: models.CodeUnitKindFunction, Exported: true},
			// Control: a non-tooling path should still flag.
			{Name: "fetchData", Path: "src/services/api.js", Kind: models.CodeUnitKindFunction, Exported: true},
		},
	}

	d := &UntestedExportDetector{}
	signals := d.Detect(snap)

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (only the non-tooling control), got %d: %v", len(signals), signalSymbols(signals))
	}
	if signals[0].Location.Symbol != "fetchData" {
		t.Errorf("expected fetchData control, got %q", signals[0].Location.Symbol)
	}
}

func TestIsToolingPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want bool
	}{
		// Tooling paths (excluded).
		{".github/actions/deploy.ts", true},
		{".gitlab/ci/build.py", true},
		{"scripts/release.sh", true},
		{"tools/codegen/main.go", true},
		{"build-tools/lint.js", true},
		{"bin/terrain.go", true},
		{"benchmarks/run.go", true},
		{"examples/usage.py", true},
		// Monorepo: tooling under a package is still tooling.
		{"packages/foo/scripts/build.ts", true},
		{"apps/bar/.github/actions/release.ts", true},
		// Production paths (kept).
		{"src/auth.ts", false},
		{"lib/parser.go", false},
		{"packages/foo/src/index.ts", false},
		{"internal/quality/untested_export.go", false},
		// Edge: "build/" in a name but not as a directory.
		{"src/buildHelpers.ts", false},
		{"src/scripts.ts", false},
		// Vendored / third-party (excluded).
		{"vendor/github.com/foo/bar.go", true},
		{"third_party/abseil/base/log.h", true},
		{"node_modules/lodash/index.js", true},
		// Generated-code directories (excluded).
		{"api/gen/proto/go/foo.pb.go", true},
		{"generated/typescript/types.ts", true},
		{"src/protobuf/messages_pb2.py", true},
		{"autogen/registry.go", true},
		// Generated-code FILE suffixes regardless of directory (excluded).
		{"pkg/server/server.pb.go", true},
		{"src/proto/messages_pb.go", true},
		{"src/services/foo_pb2.py", true},
		{"public/app.bundle.js", true},
		{"static/main.min.js", true},
		{"lib/types.generated.ts", true},
		{"lib/api.g.dart", true},
		{"models/user.freezed.dart", true},
		// Not generated despite similar-looking names (kept).
		{"src/userPb.go", false},          // not _pb.go suffix
		{"src/protobuf-helper.go", false}, // "protobuf" prefix isn't a tooling dir
		{"src/generator.ts", false},       // not _generated.* suffix
	}
	for _, c := range cases {
		got := isToolingPath(c.path)
		if got != c.want {
			t.Errorf("isToolingPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// signalSymbols pulls just the symbol field for compact failure messages.
func signalSymbols(sigs []models.Signal) []string {
	out := make([]string, 0, len(sigs))
	for _, s := range sigs {
		out = append(out, s.Location.Symbol)
	}
	return out
}

func firedSymbolSet(sigs []models.Signal) map[string]bool {
	m := make(map[string]bool, len(sigs))
	for _, s := range sigs {
		m[s.Location.Symbol] = true
	}
	return m
}

// U5: test-infrastructure filenames are not flagged as untested exports.
func TestUntestedExportDetector_SkipsTestInfraFilenames(t *testing.T) {
	t.Parallel()
	got := firedSymbolSet((&UntestedExportDetector{}).Detect(&models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "setupAuth", Path: "src/test-helpers.ts", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "mockDB", Path: "src/internal-for-testing.go", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "makeFix", Path: "src/data-fixture.ts", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "fetchUser", Path: "src/services/user.ts", Kind: models.CodeUnitKindFunction, Exported: true}, // control
		},
	}))
	for _, skip := range []string{"setupAuth", "mockDB", "makeFix"} {
		if got[skip] {
			t.Errorf("%s lives in a test-infra file and must be skipped", skip)
		}
	}
	if !got["fetchUser"] {
		t.Error("control fetchUser (real export) must fire")
	}
}

// U6: Go Test* functions are not flagged.
func TestUntestedExportDetector_SkipsGoTestFunctions(t *testing.T) {
	t.Parallel()
	got := firedSymbolSet((&UntestedExportDetector{}).Detect(&models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "TestAuth", Path: "src/auth_test.go", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "TestValidate", Path: "src/validators.go", Kind: models.CodeUnitKindFunction, Exported: true},
			{Name: "ProcessData", Path: "src/data.go", Kind: models.CodeUnitKindFunction, Exported: true}, // control
		},
	}))
	if got["TestAuth"] || got["TestValidate"] {
		t.Error("Go Test* functions must be skipped")
	}
	if !got["ProcessData"] {
		t.Error("control ProcessData must fire")
	}
}

// U7: JVM override methods (toString/hashCode/equals/...) are not flagged.
func TestUntestedExportDetector_SkipsJVMOverrides(t *testing.T) {
	t.Parallel()
	got := firedSymbolSet((&UntestedExportDetector{}).Detect(&models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "toString", Path: "src/Entity.java", Kind: models.CodeUnitKindMethod, Exported: true},
			{Name: "hashCode", Path: "src/Value.kt", Kind: models.CodeUnitKindMethod, Exported: true},
			{Name: "processPayment", Path: "src/Pay.java", Kind: models.CodeUnitKindMethod, Exported: true}, // control
		},
	}))
	if got["toString"] || got["hashCode"] {
		t.Error("JVM override methods must be skipped")
	}
	if !got["processPayment"] {
		t.Error("control processPayment must fire")
	}
}

// U8: TS/JS type/schema declarations (UppercaseFooSchema/Props/Type/...) skipped.
func TestUntestedExportDetector_SkipsTypeSchemaDeclarations(t *testing.T) {
	t.Parallel()
	got := firedSymbolSet((&UntestedExportDetector{}).Detect(&models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "UserSchema", Path: "src/user.ts", Kind: models.CodeUnitKindUnknown, Exported: true},
			{Name: "ButtonProps", Path: "src/Button.tsx", Kind: models.CodeUnitKindUnknown, Exported: true},
			{Name: "validateUser", Path: "src/validators.ts", Kind: models.CodeUnitKindFunction, Exported: true}, // control (camelCase)
		},
	}))
	if got["UserSchema"] || got["ButtonProps"] {
		t.Error("uppercase *Schema/*Props declarations must be skipped")
	}
	if !got["validateUser"] {
		t.Error("control validateUser must fire")
	}
}

// U9/U12: an import-graph link suppresses the export, and findings produced
// when an import graph is consulted carry the higher 0.7/Moderate confidence.
func TestUntestedExportDetector_ImportGraphSuppressesAndRaisesConfidence(t *testing.T) {
	t.Parallel()
	// The test file's stem ("runner") and dir ("test") deliberately match
	// neither export, so the ONLY thing that can suppress fetchUser is the
	// import-graph link — isolating that clause from the proximity heuristic.
	signals := (&UntestedExportDetector{}).Detect(&models.TestSuiteSnapshot{
		TestFiles: []models.TestFile{{Path: "test/runner.spec.ts"}},
		CodeUnits: []models.CodeUnit{
			{Name: "fetchUser", Path: "src/api.ts", Kind: models.CodeUnitKindFunction, Exported: true},   // imported → suppressed
			{Name: "fetchPost", Path: "lib/posts.ts", Kind: models.CodeUnitKindFunction, Exported: true}, // not imported → fires
		},
		ImportGraph: map[string]map[string]bool{
			"test/runner.spec.ts": {"src/api.ts": true},
		},
	})
	got := firedSymbolSet(signals)
	if got["fetchUser"] {
		t.Error("fetchUser is imported by a test → must be suppressed")
	}
	if !got["fetchPost"] {
		t.Fatal("fetchPost is not imported → must fire")
	}
	for _, s := range signals {
		if s.Location.Symbol == "fetchPost" {
			if s.Confidence != 0.7 {
				t.Errorf("import-graph confidence: want 0.7, got %v", s.Confidence)
			}
			if s.EvidenceStrength != models.EvidenceModerate {
				t.Errorf("import-graph evidence: want Moderate, got %v", s.EvidenceStrength)
			}
		}
	}
}

// U12/U13: strengthen the original test (which asserts only Type+Symbol) — pin
// the category/severity/confidence/evidence and that the explanation names the
// symbol, on the heuristic (no import graph) path.
func TestUntestedExportDetector_SignalShape(t *testing.T) {
	t.Parallel()
	signals := (&UntestedExportDetector{}).Detect(&models.TestSuiteSnapshot{
		CodeUnits: []models.CodeUnit{
			{Name: "uploadFile", Path: "src/upload.ts", Kind: models.CodeUnitKindFunction, Exported: true},
		},
	})
	if len(signals) != 1 {
		t.Fatalf("want 1 signal, got %d", len(signals))
	}
	s := signals[0]
	if s.Category != models.CategoryQuality {
		t.Errorf("category: want Quality, got %v", s.Category)
	}
	if s.Severity != models.SeverityMedium {
		t.Errorf("severity: want Medium, got %v", s.Severity)
	}
	if s.Confidence != 0.5 || s.EvidenceStrength != models.EvidenceWeak {
		t.Errorf("heuristic path: want 0.5/Weak, got %v/%v", s.Confidence, s.EvidenceStrength)
	}
	if !strings.Contains(s.Explanation, "uploadFile") {
		t.Errorf("explanation must name the symbol; got %q", s.Explanation)
	}
}
