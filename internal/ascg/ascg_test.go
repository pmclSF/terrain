package ascg

import (
	"testing"
)

func TestClassify_DocstringIsCatalog(t *testing.T) {
	r := Classify(Location{Path: "src/app.py", InDocstring: true})
	if r.Class != CatalogOrExample {
		t.Errorf("docstring should classify as catalog/example, got %v", r.Class)
	}
}

func TestClassify_CommentIsCatalog(t *testing.T) {
	r := Classify(Location{Path: "src/app.py", InComment: true})
	if r.Class != CatalogOrExample {
		t.Errorf("comment should classify as catalog/example, got %v", r.Class)
	}
}

func TestClassify_CatalogListIsCatalog(t *testing.T) {
	r := Classify(Location{Path: "src/app.py", InCatalogList: true})
	if r.Class != CatalogOrExample {
		t.Errorf("catalog list should classify as catalog/example, got %v", r.Class)
	}
}

func TestClassify_MarkdownIsCatalog(t *testing.T) {
	cases := []string{
		"README.md",
		"docs/api.md",
		"changelog.MD",
		"notebook.ipynb",
		"guide.rst",
		"index.mdx",
	}
	for _, path := range cases {
		r := Classify(Location{Path: path})
		if r.Class != CatalogOrExample {
			t.Errorf("%s should classify as catalog/example, got %v", path, r.Class)
		}
	}
}

func TestClassify_DocsPathIsCatalog(t *testing.T) {
	cases := []string{
		"docs/usage.py",
		"examples/quickstart.py",
		"sample/main.py",
		"demos/chatbot.py",
		"cookbook/embeddings.py",
		"notebooks/eval.py",
		"tutorial/intro.py",
	}
	for _, path := range cases {
		r := Classify(Location{Path: path})
		if r.Class != CatalogOrExample {
			t.Errorf("%s should classify as catalog/example, got %v", path, r.Class)
		}
	}
}

func TestClassify_FixturePathIsCatalog(t *testing.T) {
	cases := []string{
		"tests/fixtures/sample.yaml",
		"src/testdata/cfg.yaml",
		"__fixtures__/data.json",
		"__snapshots__/render.snap",
		"recordings/api.yaml",
		"tests/cassettes/get_user.yaml",
	}
	for _, path := range cases {
		r := Classify(Location{Path: path})
		if r.Class != CatalogOrExample {
			t.Errorf("%s should classify as catalog/example, got %v", path, r.Class)
		}
	}
}

func TestClassify_LiveConfigFileIsLive(t *testing.T) {
	cases := []string{
		".env",
		".env.production",
		"settings.py",
		"config.yaml",
		"config/prod.yaml",
		"environments/staging.yaml",
		"pyproject.toml",
	}
	for _, path := range cases {
		r := Classify(Location{Path: path})
		if r.Class != Live {
			t.Errorf("%s should classify as live, got %v (reasons=%v)", path, r.Class, r.Reasons)
		}
	}
}

func TestClassify_ReachedByLoaderIsLive(t *testing.T) {
	r := Classify(Location{Path: "src/app.py", ReachedByLoader: true})
	if r.Class != Live {
		t.Errorf("ReachedByLoader=true should classify as live, got %v", r.Class)
	}
}

func TestClassify_CatalogWinsOverLive(t *testing.T) {
	// A markdown file that is somehow ReachedByLoader (e.g. someone reads
	// README.md at runtime) should still classify as catalog — the false
	// positive cost of flagging readme content is worse than the false
	// negative of missing an exotic runtime use case.
	r := Classify(Location{Path: "README.md", ReachedByLoader: true})
	if r.Class != CatalogOrExample {
		t.Errorf("README.md (even with loader hint) should classify as catalog/example, got %v", r.Class)
	}
}

func TestClassify_NoSignalIsUnknown(t *testing.T) {
	r := Classify(Location{Path: "src/internal/util.py"})
	if r.Class != Unknown {
		t.Errorf("plain source file should classify as unknown, got %v (reasons=%v)", r.Class, r.Reasons)
	}
}

func TestClassify_ReasonsArePopulated(t *testing.T) {
	r := Classify(Location{Path: "docs/api.md", InDocstring: true})
	if len(r.Reasons) < 2 {
		t.Errorf("expected ≥2 reasons (path + docstring), got %v", r.Reasons)
	}
}

func TestClassify_PathSeparatorAgnostic(t *testing.T) {
	// Should work with Windows-style or POSIX-style paths.
	r := Classify(Location{Path: `docs\api.md`})
	if r.Class != CatalogOrExample {
		t.Errorf("Windows-style path should classify as catalog, got %v", r.Class)
	}
}

func TestClassification_String(t *testing.T) {
	cases := map[Classification]string{
		Unknown:          "unknown",
		Live:             "live",
		CatalogOrExample: "catalog_or_example",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("Classification(%d).String() = %q, want %q", c, got, want)
		}
	}
}
