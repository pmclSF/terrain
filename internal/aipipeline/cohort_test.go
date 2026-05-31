package aipipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectCohort_FromShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		shape RepoShape
		want  Cohort
	}{
		{
			name:  "rag-app via prompts+evals",
			shape: RepoShape{HasPromptsDir: true, HasEvalsDir: true},
			want:  CohortRAGApp,
		},
		{
			name:  "agent-app via agents dir",
			shape: RepoShape{HasAgentsDir: true},
			want:  CohortAgentApp,
		},
		{
			name:  "python library — release wf + pyproject name",
			shape: RepoShape{HasPyprojectName: true, HasReleaseWf: true},
			want:  CohortLibrarySDK,
		},
		{
			name: "node library — package.json name, non-private, release wf",
			shape: RepoShape{HasPackageJson: true, HasPackageJsonName: true,
				HasPackageJsonPriv: false, HasReleaseWf: true},
			want: CohortLibrarySDK,
		},
		{
			name:  "rust library — Cargo.toml is the strong signal",
			shape: RepoShape{HasCargoToml: true},
			want:  CohortLibrarySDK,
		},
		{
			name:  "ruby gem — gemspec is the strong signal",
			shape: RepoShape{HasGemspec: true},
			want:  CohortLibrarySDK,
		},
		{
			name:  "ml-pipeline — pipelines dir + main.py",
			shape: RepoShape{HasPipelinesDir: true, HasMainPy: true},
			want:  CohortMLPipeline,
		},
		{
			name:  "notebook-heavy — notebooks but no app entry",
			shape: RepoShape{HasNotebooksDir: true},
			want:  CohortNotebookHeavy,
		},
		{
			name:  "ai-feature-in-app — dockerfile + main.py",
			shape: RepoShape{HasDockerfile: true, HasMainPy: true},
			want:  CohortAIFeatureInApp,
		},
		{
			name: "ai-feature-in-app — node entry + vercel",
			shape: RepoShape{HasVercelJson: true, HasIndexTS: true,
				HasPackageJsonPriv: true},
			want: CohortAIFeatureInApp,
		},
		{
			name:  "ai-feature-in-app — go entry",
			shape: RepoShape{HasMainGo: true, HasDockerfile: true},
			want:  CohortAIFeatureInApp,
		},
		{
			name:  "unknown when no signals",
			shape: RepoShape{},
			want:  CohortUnknown,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectCohort(c.shape)
			if got != c.want {
				t.Errorf("DetectCohort(%+v) = %q; want %q", c.shape, got, c.want)
			}
		})
	}
}

func TestDetectCohortFromDir_RAGApp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "prompts"))
	mustMkdir(t, filepath.Join(dir, "evals"))

	cohort, shape, err := DetectCohortFromDir(dir)
	if err != nil {
		t.Fatalf("DetectCohortFromDir: %v", err)
	}
	if cohort != CohortRAGApp {
		t.Errorf("expected rag-app cohort; got %q (shape=%+v)", cohort, shape)
	}
}

func TestDetectCohortFromDir_NodeApp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "package.json"),
		`{"name":"my-app","private":true}`)
	mustWriteFile(t, filepath.Join(dir, "server.ts"), "// server")
	mustWriteFile(t, filepath.Join(dir, "Dockerfile"), "FROM node:20")

	cohort, shape, err := DetectCohortFromDir(dir)
	if err != nil {
		t.Fatalf("DetectCohortFromDir: %v", err)
	}
	if cohort != CohortAIFeatureInApp {
		t.Errorf("expected ai-feature-in-app; got %q (shape=%+v)", cohort, shape)
	}
	if !shape.HasPackageJsonPriv {
		t.Errorf("expected HasPackageJsonPriv true; got false")
	}
	if !shape.HasServerTS {
		t.Errorf("expected HasServerTS true")
	}
}

func TestDetectCohortFromDir_GoLibrary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module github.com/example/foo\n")
	wf := filepath.Join(dir, ".github", "workflows")
	mustMkdir(t, wf)
	mustWriteFile(t, filepath.Join(wf, "release.yml"), "name: release")

	cohort, shape, err := DetectCohortFromDir(dir)
	if err != nil {
		t.Fatalf("DetectCohortFromDir: %v", err)
	}
	if cohort != CohortLibrarySDK {
		t.Errorf("expected library-sdk for Go module with release wf; got %q (shape=%+v)",
			cohort, shape)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func mustWriteFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}
