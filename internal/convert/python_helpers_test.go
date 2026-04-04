package convert

import (
	"reflect"
	"testing"
)

func TestParsePythonBlocks_PreservesStructuredBlocksAndRawSections(t *testing.T) {
	t.Parallel()

	source := `import pytest

@pytest.fixture
def client():
    yield make_client()

class TestAuth:
    def test_login(self):
        assert True
`

	blocks := parsePythonBlocks(source, 0)
	if len(blocks) != 3 {
		t.Fatalf("len(blocks) = %d, want 3", len(blocks))
	}
	if blocks[0].Kind != "raw" || !reflect.DeepEqual(blocks[0].Raw, []string{"import pytest", ""}) {
		t.Fatalf("unexpected raw block: %#v", blocks[0])
	}
	if blocks[1].Kind != "function" || blocks[1].Signature != "def client():" {
		t.Fatalf("unexpected function block: %#v", blocks[1])
	}
	if !reflect.DeepEqual(blocks[1].Decorators, []string{"@pytest.fixture"}) {
		t.Fatalf("decorators = %#v, want fixture decorator", blocks[1].Decorators)
	}
	if !reflect.DeepEqual(blocks[1].Body, []string{"yield make_client()", ""}) {
		t.Fatalf("function body = %#v, want dedented body", blocks[1].Body)
	}
	if blocks[2].Kind != "class" || blocks[2].Signature != "class TestAuth:" {
		t.Fatalf("unexpected class block: %#v", blocks[2])
	}
}

func TestExtractPythonFuncParts_SplitsTopLevelParams(t *testing.T) {
	t.Parallel()

	name, params, async := extractPythonFuncParts(`async def test_api(client, payload={"a": [1, 2]}, *, retries=3):`)
	if name != "test_api" {
		t.Fatalf("name = %q, want test_api", name)
	}
	if !async {
		t.Fatal("expected async=true")
	}
	wantParams := []string{"client", `payload={"a": [1, 2]}`, "*", "retries=3"}
	if !reflect.DeepEqual(params, wantParams) {
		t.Fatalf("params = %#v, want %#v", params, wantParams)
	}
}

func TestSplitFixtureBodyAroundYield(t *testing.T) {
	t.Parallel()

	before, after, hasYield := splitFixtureBodyAroundYield([]string{
		"db = connect()",
		"yield",
		"db.close()",
	})
	if !hasYield {
		t.Fatal("expected hasYield=true")
	}
	if !reflect.DeepEqual(before, []string{"db = connect()"}) {
		t.Fatalf("before = %#v, want setup body", before)
	}
	if !reflect.DeepEqual(after, []string{"db.close()"}) {
		t.Fatalf("after = %#v, want teardown body", after)
	}
}

func TestBuildPytestDecoratorFromUnittest_HandlesSkipIf(t *testing.T) {
	t.Parallel()

	got, ok := buildPytestDecoratorFromUnittest(`@unittest.skipIf(sys.platform == "win32", "windows-only")`)
	if !ok {
		t.Fatal("expected skipIf decorator conversion")
	}
	want := `@pytest.mark.skipif(sys.platform == "win32", reason="windows-only")`
	if got != want {
		t.Fatalf("decorator = %q, want %q", got, want)
	}
}

func TestParsePytestParametrizeDecorator_HandlesStringParamList(t *testing.T) {
	t.Parallel()

	names, expr, ok := parsePytestParametrizeDecorator(`@pytest.mark.parametrize("value, expected", [(1, 2), (3, 4)])`)
	if !ok {
		t.Fatal("expected parametrize decorator conversion")
	}
	wantNames := []string{"value", "expected"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("names = %#v, want %#v", names, wantNames)
	}
	if expr != "[(1, 2), (3, 4)]" {
		t.Fatalf("expr = %q, want %q", expr, "[(1, 2), (3, 4)]")
	}
}

func TestSplitPythonBinaryExpr_RespectsNestingAndQuotes(t *testing.T) {
	t.Parallel()

	left, right, ok := splitPythonBinaryExpr(`payload["a,b"] == func(1, 2)`, "==")
	if !ok {
		t.Fatal("expected splitPythonBinaryExpr to succeed")
	}
	if left != `payload["a,b"]` || right != `func(1, 2)` {
		t.Fatalf("split = (%q, %q), want (%q, %q)", left, right, `payload["a,b"]`, `func(1, 2)`)
	}
}
