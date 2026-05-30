package quality

import (
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
	// Verified-on-corpus FP: Angular's .github/actions/deploy-docs-site/
	// flagged for "untested" exports. Detector now skips these paths
	// by default — deploy scripts don't need unit-test coverage.
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
