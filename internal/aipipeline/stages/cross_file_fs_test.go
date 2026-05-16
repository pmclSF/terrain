package stages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSResolver_SiblingMlflow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "src/train/model.py", "import sklearn\nclf.fit(X, y)\n")
	mkfile(t, root, "src/train/track.py", "import mlflow\nmlflow.log_metric('acc', 0.9)\n")
	r := NewFSResolver(root)
	if !r.SiblingHasEvalMarker("src/train/model.py") {
		t.Errorf("expected sibling mlflow import to register as eval marker")
	}
}

func TestFSResolver_NoSiblingMarker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "src/train/model.py", "import sklearn\n")
	mkfile(t, root, "src/train/loader.py", "import numpy\n")
	r := NewFSResolver(root)
	if r.SiblingHasEvalMarker("src/train/model.py") {
		t.Errorf("did not expect eval marker — siblings only have numpy/sklearn")
	}
}

func TestFSResolver_PackageMarkerOneLevelUp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "src/train/__init__.py", "")
	mkfile(t, root, "src/train/model.py", "import sklearn\n")
	mkfile(t, root, "src/train/evals/eval_metrics.py",
		"from deepeval.metrics import AnswerRelevancy\n")
	r := NewFSResolver(root)
	if !r.PackageHasEvalMarker("src/train/model.py") {
		t.Errorf("expected deepeval import in evals/ subdirectory to register")
	}
}

func TestFSResolver_SkipsNodeModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "package.json", "{}")
	mkfile(t, root, "src/agent/index.ts", "import OpenAI from 'openai';\n")
	mkfile(t, root, "node_modules/pytest/index.js", "import pytest from 'pytest';\n")
	r := NewFSResolver(root)
	if r.PackageHasEvalMarker("src/agent/index.ts") {
		t.Errorf("node_modules pytest import must not register; walk should skip the dir")
	}
}

func TestFSResolver_ExcludesSelf(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// The candidate file itself imports pytest. SiblingHasEvalMarker
	// must not see this — only siblings count.
	mkfile(t, root, "src/agent/handler.py", "import pytest\n")
	r := NewFSResolver(root)
	if r.SiblingHasEvalMarker("src/agent/handler.py") {
		t.Errorf("candidate file's own imports must not count as sibling markers")
	}
}

func mkfile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
