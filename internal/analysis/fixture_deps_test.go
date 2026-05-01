package analysis

import (
	"reflect"
	"testing"
)

func TestExtractPyFixtureDeps_Basic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "no params",
			line: `def alone():`,
			want: nil,
		},
		{
			name: "single dep",
			line: `def with_db(db):`,
			want: []string{"db"},
		},
		{
			name: "multiple deps",
			line: `def fixture(db, redis, queue):`,
			want: []string{"db", "redis", "queue"},
		},
		{
			name: "drops request and pytest builtins",
			line: `def w(db, request, tmp_path, monkeypatch, redis):`,
			want: []string{"db", "redis"},
		},
		{
			name: "method receiver self filtered",
			line: `def fixture(self, db):`,
			want: []string{"db"},
		},
		{
			name: "default values stripped",
			line: `def db(scope="session", maker=None):`,
			want: []string{"scope", "maker"},
		},
		{
			name: "type annotations stripped",
			line: `def with_db(db: Database, redis: Redis):`,
			want: []string{"db", "redis"},
		},
		{
			name: "varargs and kwargs dropped",
			line: `def fixture(db, *args, **kwargs):`,
			want: []string{"db"},
		},
		{
			name: "async def",
			line: `async def afixture(db, redis):`,
			want: []string{"db", "redis"},
		},
		{
			name: "non-def line returns nil",
			line: `if pending:`,
			want: nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractPyFixtureDeps(tc.line)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("extractPyFixtureDeps(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestExtractPyFixtureDeps_PicksUpInFixtureExtractor(t *testing.T) {
	t.Parallel()

	src := `
import pytest

@pytest.fixture
def db():
    return Database()

@pytest.fixture(scope="session")
def authenticated_user(db, request):
    return create_user(db)
`
	fixtures := detectPythonFixtures(src, "tests/conftest.py", "pytest")

	byName := map[string][]string{}
	for _, f := range fixtures {
		byName[f.Name] = f.Dependencies
	}
	if deps, ok := byName["db"]; !ok || len(deps) != 0 {
		t.Errorf("db fixture deps = %v, want empty", deps)
	}
	deps, ok := byName["authenticated_user"]
	if !ok {
		t.Fatal("authenticated_user fixture not detected")
	}
	if len(deps) != 1 || deps[0] != "db" {
		t.Errorf("authenticated_user deps = %v, want [db]", deps)
	}
}
