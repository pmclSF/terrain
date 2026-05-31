package pathnoise

import "testing"

func TestIsToolingPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Production code — should NOT be tooling.
		{"src/auth/login.ts", false},
		{"internal/aidetect/hardcoded_api_key.go", false},
		{"flaml/autogen/oai/completion.py", false}, // not auto-generated

		// CI/build tooling
		{".github/workflows/ci.yml", true},
		{".circleci/config.yml", true},
		{"scripts/deploy.sh", true},
		{"tools/codegen/main.go", true},

		// Vendored
		{"vendor/github.com/pkg/errors/errors.go", true},
		{"node_modules/lodash/index.js", true},

		// Generated suffixes
		{"foo.pb.go", true},
		{"models_pb2.py", true},
		{"main.min.js", true},
		{"model.g.dart", true},
		{"data.generated.ts", true},

		// Test fixtures
		{"tests/data/sample.json", true},
		{"__fixtures__/user.json", true},
		{"src/api/__fixtures__/response.json", true},
		{"tests/fixtures/auth.json", true},
		{"path/with/cassettes/recorded.yaml", true},
		{"some/dir/placebo/iam.json", true},

		// New patterns (corpus-driven)
		{"tests-gen/Kotlin/foo.kt", true},
		{"applyconfigurations/storage/v1/types.go", true},
		{"apps/playground/lib/store.ts", true},
		{"benchmark/index.js", true},
		{"examples/quickstart.py", true},
		{"docs/api-reference.md", true},

		// Filename markers
		{"src/internal-for-testing.ts", true},
		{"pkg/api-fixture.go", true},
		{"src/user.fixture.ts", true},
	}
	for _, c := range cases {
		got := IsToolingPath(c.path)
		if got != c.want {
			t.Errorf("IsToolingPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsTestPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"src/auth/login.ts", false},
		{"src/auth/login.test.ts", true},
		{"tests/auth_test.py", true},
		{"test/handler_test.go", true},
		{"foo_test.go", true},
		{"src/foo.spec.ts", true},
		{"production/path.go", false},
	}
	for _, c := range cases {
		got := IsTestPath(c.path)
		if got != c.want {
			t.Errorf("IsTestPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
